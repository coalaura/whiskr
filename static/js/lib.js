function storeValue(key, value) {
	if (!value) {
		localStorage.removeItem(key);

		return;
	}

	localStorage.setItem(key, JSON.stringify(value));
}

function loadValue(key, fallback = false) {
	const raw = localStorage.getItem(key);

	if (!raw) {
		return fallback;
	}

	try {
		const value = JSON.parse(raw);

		if (!value) {
			throw new Error("no value");
		}

		return value;
	} catch {}

	return fallback;
}

function schedule(cb) {
	if (document.visibilityState === "visible") {
		requestAnimationFrame(cb);

		return;
	}

	setTimeout(cb, 80);
}

function uid() {
	return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function make(tag, ...classes) {
	const el = document.createElement(tag);

	if (classes.length) {
		el.classList.add(...classes);
	}

	return el;
}

function escapeHtml(text) {
	return text
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;");
}
