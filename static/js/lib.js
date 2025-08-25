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

function fillSelect($select, options, callback) {
	$select.innerHTML = "";

	for (const option of options) {
		const el = document.createElement("option");

		callback(el, option);

		$select.appendChild(el);
	}
}

function wait(ms) {
	return new Promise(resolve => setTimeout(resolve, ms));
}

function escapeHtml(text) {
	return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
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

function formatMoney(num) {
	if (num === 0) {
		return "0ct";
	}

	if (num < 1) {
		let decimals = 1;

		if (num < 0.0001) {
			decimals = 3;
		} else if (num < 0.001) {
			decimals = 2;
		}

		return `${fixed(num * 100, decimals)}ct`;
	}

	return `$${fixed(num, 2)}`;
}

function clamp(num, min, max) {
	return Math.min(Math.max(num, min), max);
}

function download(name, type, data) {
	let blob;

	if (data instanceof Blob) {
		blob = data;
	} else {
		blob = new Blob([data], {
			type: type,
		});
	}

	const a = document.createElement("a"),
		url = URL.createObjectURL(blob);

	a.setAttribute("download", name);
	a.style.display = "none";
	a.href = url;

	document.body.appendChild(a);

	a.click();

	document.body.removeChild(a);
	URL.revokeObjectURL(url);
}

function lines(text) {
	let count = 0,
		index = 0;

	while (index < text.length) {
		index = text.indexOf("\n", index);

		if (index === -1) {
			break;
		}

		count++;
		index++;
	}

	return count + 1;
}

function selectFile(accept, asJson = false) {
	return new Promise(resolve => {
		const input = make("input");

		input.type = "file";
		input.accept = accept;

		input.onchange = () => {
			const file = input.files[0];

			if (!file) {
				resolve(false);

				return;
			}

			const reader = new FileReader();

			reader.onload = () => {
				let content = reader.result;

				if (asJson) {
					try {
						content = JSON.parse(content);
					} catch {
						resolve(false);

						return;
					}
				}

				resolve({
					name: file.name,
					content: content,
				});
			};

			reader.onerror = () => resolve(false);

			reader.readAsText(file);
		};

		input.click();
	});
}
