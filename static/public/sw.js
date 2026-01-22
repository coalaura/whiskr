const Prefix = "/-/local/",
	MaxItems = 64;

const store = new Map();

function cleanup() {
	if (store.size <= MaxItems) {
		return;
	}

	const entries = [...store.entries()].sort((a, b) => a[1].ts - b[1].ts);

	while (entries.length && store.size > MaxItems) {
		const [key] = entries.shift();
		store.delete(key);
	}
}

self.addEventListener("install", event => {
	event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", event => {
	event.waitUntil(self.clients.claim());
});

self.addEventListener("message", event => {
	const msg = event.data;

	if (!msg) {
		return;
	}

	if (msg.type === "whiskr:ping") {
		self.clients.claim();

		return;
	}

	if (msg.type !== "whiskr:image-put") {
		return;
	}

	const { key, mime, bytes } = msg;

	if (!key || typeof key !== "string") {
		return postError(event, "missing key");
	}

	if (!mime || typeof mime !== "string") {
		return postError(event, "missing mime");
	}

	if (!(bytes instanceof ArrayBuffer)) {
		return postError(event, "missing bytes");
	}

	store.set(key, {
		mime: mime,
		bytes: bytes,
		ts: Date.now(),
	});

	cleanup();

	event.ports?.[0]?.postMessage({
		ok: true,
	});
});

self.addEventListener("fetch", event => {
	const url = new URL(event.request.url);

	if (!url.pathname.startsWith(Prefix)) {
		return;
	}

	event.respondWith(
		(async () => {
			const key = decodeURIComponent(url.pathname.slice(Prefix.length));
			const entry = store.get(key);

			if (!entry) {
				return new Response("not found", {
					status: 404,
					headers: {
						"Content-Type": "text/plain; charset=utf-8",
						"Cache-Control": "no-store",
					},
				});
			}

			entry.ts = Date.now();

			const headers = new Headers();

			headers.set("Content-Type", entry.mime);
			headers.set("Content-Disposition", `${url.searchParams.has("download") ? "attachment" : "inline"}; filename="${key}"`);
			headers.set("Cache-Control", "private, max-age=0, no-store");

			return new Response(entry.bytes, {
				status: 200,
				headers: headers,
			});
		})()
	);
});

function postError(event, msg) {
	event.ports?.[0]?.postMessage({
		ok: false,
		error: msg,
	});
}
