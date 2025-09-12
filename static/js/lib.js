/** biome-ignore-all lint/correctness/noUnusedVariables: utility */

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
	classes = classes.filter(Boolean);

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

function dataBlob(dataUrl) {
	const [header, data] = dataUrl.split(","),
		mime = header.match(/data:(.*?)(;|$)/)[1];

	let blob;

	if (header.includes(";base64")) {
		const bytes = atob(data),
			numbers = new Array(bytes.length);

		for (let i = 0; i < bytes.length; i++) {
			numbers[i] = bytes.charCodeAt(i);
		}

		const byteArray = new Uint8Array(numbers);

		blob = new Blob([byteArray], {
			type: mime,
		});
	} else {
		const text = decodeURIComponent(data);

		blob = new Blob([text], {
			type: mime,
		});
	}

	return blob;
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

function wrapJSON(txt) {
	if (!txt || !txt.startsWith("{")) {
		return txt;
	}

	try {
		const data = JSON.parse(txt);

		return `\`\`\`json\n${JSON.stringify(data, null, 2)}\n\`\`\``;
	} catch {}

	return txt;
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

function readFile(file, handler, onError = false) {
	return new Promise(resolve => {
		const reader = new FileReader();

		reader.onload = () => {
			try {
				const result = {
					name: file.name,
					content: reader.result,
				};

				handler(result);

				resolve(result);
			} catch (err) {
				onError?.(`${file.name}: ${err.message}`);

				resolve(false);
			}
		};

		reader.onerror = () => resolve(false);

		reader.readAsText(file);
	});
}

function selectFile(accept, multiple, handler, onError = false) {
	return new Promise(resolve => {
		const input = make("input");

		input.type = "file";
		input.accept = accept;
		input.multiple = multiple;

		input.onchange = async () => {
			const files = input.files;

			if (!files.length) {
				resolve(false);

				return;
			}

			const results = [];

			for (const file of files) {
				const result = await readFile(file, handler, onError);

				if (result) {
					results.push(result);
				}
			}

			if (!results.length) {
				resolve(false);

				return;
			}

			resolve(multiple ? results : results[0]);
		};

		input.click();
	});
}

async function detectPlatform() {
	let os, arch;

	let platform = navigator.platform || "";

	if (navigator.userAgentData?.getHighEntropyValues) {
		try {
			const data = await navigator.userAgentData.getHighEntropyValues(["platform", "architecture"]);

			platform = data.platform;
			arch = data.architecture;
		} catch {}
	}

	const ua = navigator.userAgent || "";

	// Windows
	if (/Windows NT 10\.0/.test(ua)) os = "Windows 10/11";
	else if (/Windows NT 6\.3/.test(ua)) os = "Windows 8.1";
	else if (/Windows NT 6\.2/.test(ua)) os = "Windows 8";
	else if (/Windows NT 6\.1/.test(ua)) os = "Windows 7";
	else if (/Windows NT 6\.0/.test(ua)) os = "Windows Vista";
	else if (/Windows NT 5\.1/.test(ua)) os = "Windows XP";
	else if (/Windows NT 5\.0/.test(ua)) os = "Windows 2000";
	else if (/Windows NT 4\.0/.test(ua)) os = "Windows NT 4.0";
	else if (/Win(98|95|16)/.test(ua)) os = "Windows (legacy)";
	else if (/Windows/.test(ua)) os = "Windows (unknown version)";
	// Mac OS
	else if (/Mac OS X/.test(ua)) {
		os = "macOS";

		const match = ua.match(/Mac OS X ([0-9_]+)/);

		if (match) {
			os += ` ${match[1].replace(/_/g, ".")}`;
		} else {
			os += " (unknown version)";
		}
	}
	// Chrome OS
	else if (/CrOS/.test(ua)) {
		os = "Chrome OS";

		const match = ua.match(/CrOS [^ ]+ ([0-9.]+)/);

		if (match) {
			os += ` ${match[1]}`;
		}
	}
	// Linux (special)
	else if (/FreeBSD/.test(ua)) os = "FreeBSD";
	else if (/OpenBSD/.test(ua)) os = "OpenBSD";
	else if (/NetBSD/.test(ua)) os = "NetBSD";
	else if (/SunOS/.test(ua)) os = "Solaris";
	// Linux (generic)
	else if (/Linux/.test(ua)) {
		if (/Ubuntu/i.test(ua)) os = "Ubuntu";
		else if (/Debian/i.test(ua)) os = "Debian";
		else if (/Fedora/i.test(ua)) os = "Fedora";
		else if (/CentOS/i.test(ua)) os = "CentOS";
		else if (/Red Hat/i.test(ua)) os = "Red Hat";
		else if (/SUSE/i.test(ua)) os = "SUSE";
		else if (/Gentoo/i.test(ua)) os = "Gentoo";
		else if (/Arch/i.test(ua)) os = "Arch Linux";
		else os = "Linux";
	}
	// Mobile
	else if (/Android/.test(ua)) os = "Android";
	else if (/iPhone|iPad|iPod/.test(ua)) os = "iOS";

	// We still have no OS?
	if (!os && platform) {
		if (platform.includes("Win")) os = "Windows";
		else if (/Mac/.test(platform)) os = "macOS";
		else if (/Linux/.test(platform)) os = "Linux";
		else os = platform;
	}

	// Detect architecture
	if (!arch) {
		if (/WOW64|Win64|x64|amd64/i.test(ua)) arch = "x64";
		else if (/arm64|aarch64/i.test(ua)) arch = "arm64";
		else if (/i[0-9]86|x86/i.test(ua)) arch = "x86";
		else if (/ppc/i.test(ua)) arch = "ppc";
		else if (/sparc/i.test(ua)) arch = "sparc";
		else if (platform && /arm/i.test(platform)) arch = "arm";
	}

	return `${os || "Unknown OS"}${arch ? `, ${arch}` : ""}`;
}

(() => {
	const $notifications = document.getElementById("notifications");

	window.notify = async (msg, persistent = false) => {
		console.warn(msg);

		const notification = make("div", "notification", "off-screen");

		notification.textContent = msg instanceof Error ? msg.message : msg;

		$notifications.appendChild(notification);

		await wait(250);

		notification.classList.remove("off-screen");

		if (persistent) {
			return;
		}

		await wait(5000);

		notification.style.height = `${notification.getBoundingClientRect().height}px`;

		notification.classList.add("off-screen");

		await wait(250);

		notification.remove();
	};
})();
