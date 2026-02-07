export function schedule(cb) {
	if (document.visibilityState === "visible") {
		requestAnimationFrame(cb);

		return;
	}

	setTimeout(cb, 80);
}

export function uid() {
	return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

export function make(tag, ...classes) {
	classes = classes.filter(Boolean);

	const el = document.createElement(tag);

	if (classes.length) {
		el.classList.add(...classes);
	}

	return el;
}

export function bHeight(el) {
	return el.getBoundingClientRect().height;
}

export function fillSelect($select, options, callback) {
	$select.innerHTML = "";

	for (const option of options) {
		const el = document.createElement("option");

		callback(el, option);

		$select.appendChild(el);
	}
}

export function wait(ms) {
	return new Promise(resolve => setTimeout(resolve, ms));
}

export function escapeHtml(text) {
	return text.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const fracZerosRgx = /(?:(\.\d*?[1-9])0+|\.0+)$/;

export function round(num, digits) {
	return num.toFixed(digits).replace(fracZerosRgx, "$1") || "0";
}

export function formatMilliseconds(ms) {
	if (ms < 1000) {
		return `${ms}ms`;
	}

	if (ms < 60000) {
		return `${round(ms / 1000, 1)}s`;
	}

	const minutes = Math.floor(ms / 60000),
		seconds = ms - minutes * 60000;

	return `${minutes}m ${round(seconds / 1000, 1)}s`;
}

export function formatTimestamp(ts) {
	return new Date(ts * 1000).toLocaleDateString();
}

export function formatBytes(bytes) {
	if (!+bytes) {
		return "0B";
	}

	const sizes = ["B", "kB", "MB", "GB", "TB"],
		i = Math.floor(Math.log(bytes) / Math.log(1000));

	const val = bytes / Math.pow(1000, i),
		dec = i === 0 ? 0 : val < 10 ? 2 : 1;

	return `${val.toFixed(dec)}${sizes[i]}`;
}

function mimeToExt(mime) {
	switch (mime) {
		case "image/png":
			return "png";
		case "image/jpeg":
			return "jpg";
		case "image/webp":
			return "webp";
	}

	return null;
}

function dataUrlToBytes(dataUrl) {
	const comma = dataUrl.indexOf(","),
		meta = dataUrl.slice(5, comma),
		mime = meta.slice(0, meta.indexOf(";")),
		b64 = dataUrl.slice(comma + 1);

	const bin = atob(b64),
		bytes = new Uint8Array(bin.length);

	for (let i = 0; i < bin.length; i++) {
		bytes[i] = bin.charCodeAt(i);
	}

	return {
		mime: mime,
		bytes: bytes,
	};
}

async function sha256Hex(bytes) {
	const digest = await crypto.subtle.digest("SHA-256", bytes),
		hashBytes = new Uint8Array(digest);

	let hex = "";

	for (const bt of hashBytes) {
		hex += bt.toString(16).padStart(2, "0");
	}

	return hex;
}

function convertToJpeg(dataUrl) {
	return new Promise((resolve, reject) => {
		const img = new Image();

		img.onload = () => {
			const canvas = document.createElement("canvas");

			canvas.width = img.width;
			canvas.height = img.height;

			const ctx = canvas.getContext("2d");

			ctx.fillStyle = "#FFFFFF";
			ctx.fillRect(0, 0, canvas.width, canvas.height);

			ctx.drawImage(img, 0, 0);

			const jpegDataUrl = canvas.toDataURL("image/jpeg", 0.9);

			resolve(jpegDataUrl);
		};

		img.onerror = () => {
			reject(new Error("Failed to load image for conversion"));
		};

		img.src = dataUrl;
	});
}

export function readFileAsDataUrl(file) {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();

		reader.onload = async () => {
			try {
				const jpegDataUrl = await convertToJpeg(reader.result);

				resolve(jpegDataUrl);
			} catch (err) {
				reject(err);
			}
		};

		reader.onerror = () => reject(new Error("Failed to read file"));

		reader.readAsDataURL(file);
	});
}

export async function dataUrlFilename(dataUrl) {
	const { mime, bytes } = dataUrlToBytes(dataUrl),
		ext = mimeToExt(mime),
		hash = await sha256Hex(bytes);

	return `${hash.slice(0, 4)}${hash.slice(-4)}.${ext}`;
}

const trailingZeroRgx = /\.?0+$/m;

export function fixed(num, decimals = 0) {
	return num.toFixed(decimals).replace(trailingZeroRgx, "");
}

export function formatMoney(num) {
	if (num === 0) {
		return "0ct";
	}

	if (num < 1) {
		let decimals = 1;

		if (num < 0.00001) {
			decimals = 4;
		} else if (num < 0.0001) {
			decimals = 3;
		} else if (num < 0.001) {
			decimals = 2;
		}

		return `${fixed(num * 100, decimals)}ct`;
	}

	return `$${fixed(num, 2)}`;
}

export function clamp(num, min, max) {
	return Math.min(Math.max(num, min), max);
}

export function wrapJSON(txt) {
	if (!txt || !txt.startsWith("{")) {
		return txt;
	}

	try {
		const data = JSON.parse(txt);

		return `\`\`\`json\n${JSON.stringify(data, null, 2)}\n\`\`\``;
	} catch {}

	return txt;
}

export function download(name, type, data) {
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

export function lines(text) {
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

export function previewFile(file) {
	// build form
	const form = make("form");

	form.style.display = "none";

	form.enctype = "multipart/form-data";
	form.method = "post";
	form.action = "/-/preview";
	form.target = "_blank";

	// add name field
	const name = make("input");

	name.name = "name";
	name.value = file.name;

	form.appendChild(name);

	// add content field
	const content = make("textarea");

	content.name = "content";
	content.value = file.content;

	form.appendChild(content);

	// send form
	document.body.appendChild(form);

	form.submit();

	form.remove();
}

export function readFile(file, handler, onError = false) {
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

export function selectFile(accept, multiple, handler, onError = false) {
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

const platformRegexes = [
	// Mobile
	[/(iPhone|iPad|iPod)/, "iOS"],
	[/Android/i, "Android"],
	[/Windows Phone|IEMobile/i, "Windows Phone"],

	// Chrome OS
	[/CrOS/i, "Chrome OS"],

	// Windows
	[/Windows NT 10\.0/, "Windows 10/11"],
	[/Windows NT 6\.3/, "Windows 8.1"],
	[/Windows NT 6\.2/, "Windows 8"],
	[/Windows NT 6\.1/, "Windows 7"],
	[/Windows NT 6\.0/, "Windows Vista"],
	[/Windows NT 5\.1/, "Windows XP"],
	[/Windows NT 5\.0/, "Windows 2000"],
	[/Windows NT 4\.0/, "Windows NT 4.0"],
	[/Win(98|95|16)/, "Windows (legacy 95/98)"],
	[/Windows/, "Windows (unknown version)"],

	// Mac
	[/Macintosh;.*Mac OS X/, "macOS"],

	// Chrome OS
	[/CrOS/, "Chrome OS"],

	// BSD/UNIX
	[/FreeBSD/, "FreeBSD"],
	[/OpenBSD/, "OpenBSD"],
	[/NetBSD/, "NetBSD"],
	[/SunOS/, "Solaris"],

	// Linux distro's
	[/Ubuntu/i, "Ubuntu"],
	[/Debian/i, "Debian"],
	[/Fedora/i, "Fedora"],
	[/CentOS/i, "CentOS"],
	[/(?:Red Hat|RHEL)/i, "Red Hat"],
	[/(?:openSUSE|SUSE|SLES)/i, "SUSE"],
	[/Gentoo/i, "Gentoo"],
	[/Arch Linux/i, "Arch Linux"],
	[/Alpine/i, "Alpine Linux"],
	[/Linux/i, "Linux"],
];

const architectureRegexes = [
	[/arm64|aarch64|armv8(?:\.\d+)?/i, "arm64"],
	[/armv?7|armhf/i, "arm"],
	[/WOW64|Win64|x64|x86_64|amd64/i, "x64"],
	[/\b(ia32|i[3-6]86|x86)\b/i, "x86"],
	[/ppc64le|powerpc64le/i, "ppc64le"],
	[/ppc64|powerpc64/i, "ppc64"],
	[/ppc|powerpc/i, "ppc"],
	[/s390x/i, "s390x"],
	[/mips64/i, "mips64"],
	[/mips/i, "mips"],
];

export async function detectPlatform() {
	let os,
		arch,
		platform = navigator.platform || "";

	if (navigator.userAgentData?.getHighEntropyValues) {
		try {
			const data = await navigator.userAgentData.getHighEntropyValues(["platform", "architecture"]);

			platform = data.platform;
			arch = data.architecture;
		} catch {}
	}

	const ua = navigator.userAgent || "";

	for (const rgx of platformRegexes) {
		if (rgx[0].test(ua)) {
			os = rgx[1];

			break;
		}
	}

	// We still have no OS?
	if (!os && platform) {
		if (platform.includes("Win")) {
			os = "Windows";
		} else if (platform.includes("Mac")) {
			os = "macOS";
		} else if (platform.includes("Linux")) {
			os = "Linux";
		} else {
			os = platform;
		}
	}

	// Detect architecture
	if (!arch) {
		for (const rgx of architectureRegexes) {
			if (rgx[0].test(ua)) {
				arch = rgx[1];

				break;
			}
		}

		if (!arch && platform?.toLowerCase()?.includes("arm")) {
			arch = "arm";
		}
	}

	return `${os || "Unknown OS"}${arch ? `, ${arch}` : ""}`;
}

const $notifications = document.getElementById("notifications");

export async function notify(msg, type = "error", persistent = false) {
	const notification = make("div", "notification", type, "off-screen");

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
}

window.notify = notify;
