/** biome-ignore-all lint/correctness/noUnusedVariables: utility */

function storeValue(key, value = false) {
	if (value === null || value === undefined || value === false) {
		localStorage.removeItem(key);

		return;
	}

	localStorage.setItem(key, JSON.stringify(value));
}

function loadValue(key, fallback = false) {
	const raw = localStorage.getItem(key);

	if (raw === null) {
		return fallback;
	}

	try {
		const value = JSON.parse(raw);

		if (value === null) {
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

function formatMilliseconds(ms) {
	if (ms < 1000) {
		return `${ms}ms`;
	} else if (ms < 10000) {
		return `${(ms / 1000).toFixed(1)}s`;
	}

	return `${Math.round(ms / 1000)}s`;
}

function fixed(num, decimals = 0) {
	return num.toFixed(decimals).replace(/\.?0+$/m, "");
}
