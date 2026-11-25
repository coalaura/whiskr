const audioExtensions = new Set(["mp3", "wav", "flac", "m4a", "ogg", "aac", "aiff"]),
	videoExtensions = new Set(["mp4", "webm", "mov", "m4v", "mpeg", "mpg", "avi", "ogv", "3gp"]),
	youtubeHosts = new Set(["youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be", "www.youtu.be"]);

export function classifyMediaURL(raw) {
	let url;

	try {
		url = new URL(raw);
	} catch {
		return null;
	}

	if (!["https:", "http:"].includes(url.protocol) || url.username || url.password) {
		return null;
	}

	if (youtubeHosts.has(url.hostname)) {
		let videoID = "";

		if (url.hostname === "youtu.be" || url.hostname === "www.youtu.be") {
			videoID = url.pathname.slice(1);
		} else if (url.pathname === "/watch") {
			videoID = url.searchParams.get("v") || "";
		} else {
			videoID = url.pathname.match(/^\/(?:shorts|embed|live)\/([^/]+)\/?$/)?.[1] || "";
		}

		videoID = videoID.replace(/\/$/, "");

		return {
			type: /^[a-zA-Z0-9_-]{11}$/.test(videoID) ? "youtube" : "link",
			videoID: videoID,
		};
	}

	let pathname;

	try {
		pathname = decodeURIComponent(url.pathname);
	} catch {
		return null;
	}

	const extension = pathname.match(/\.([a-z0-9]+)$/i)?.[1].toLowerCase();

	if (audioExtensions.has(extension)) {
		return { type: "audio" };
	}

	if (videoExtensions.has(extension)) {
		return { type: "video" };
	}

	return null;
}

export function createMediaElement(url, text, title) {
	const media = classifyMediaURL(url);

	if (!media) {
		return null;
	}

	const wrapper = document.createElement("span"),
		label = text || title || url;

	if (media.type === "link") {
		return wrapper;
	}

	wrapper.className = `no-click inline-media media-${media.type}`;

	if (media.type === "youtube") {
		const preview = document.createElement("button"),
			thumbnail = document.createElement("img");

		wrapper.dataset.youtubeId = media.videoID;
		preview.type = "button";
		preview.className = "youtube-preview";
		preview.setAttribute("aria-label", `Play on YouTube: ${label}`);

		thumbnail.alt = "";
		thumbnail.loading = "lazy";
		thumbnail.width = 1280;
		thumbnail.height = 720;
		thumbnail.src = `/-/youtube/${media.videoID}`;

		preview.append(thumbnail);
		wrapper.append(preview);
	} else if (media.type === "audio" || media.type === "video") {
		const player = document.createElement(media.type);

		player.src = url;
		player.controls = true;
		player.preload = "none";
		player.setAttribute("aria-label", label);

		if (media.type === "video") {
			player.playsInline = true;
		}

		wrapper.append(player);
	}

	const warning = document.createElement("span"),
		kind = media.type === "youtube" ? "YouTube" : media.type;

	warning.className = "media-warning";
	warning.textContent = `Model does not have ${kind} support`;

	wrapper.append(warning);

	return wrapper;
}
