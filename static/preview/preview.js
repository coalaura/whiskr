import "./preview.css";

import hljs from "highlight.js";

function init() {
	const title = document.querySelector("title"),
		body = document.querySelector("code"),
		frame = document.querySelector("iframe"),
		print = document.querySelector(".print"),
		isHtml = /\.html?$/i.test(data.name.trim());

	title.innerText = data.name;

	if (isHtml) {
		title.innerText += " (preview)";

		const layout = { a4: "A4", letter: "letter", legal: "legal" }[data.layout];

		if (layout) {
			frame.sandbox.add("allow-modals");
		}

		frame.srcdoc = layout ? addPrintLayout(data.content, layout) : data.content;
		frame.hidden = false;

		if (layout) {
			print.hidden = false;
			print.addEventListener("click", () => frame.contentWindow.postMessage("whiskr-print", "*"));
		}

		return;
	}

	let language = guessLanguage(data.name);

	if (language) {
		const code = hljs.highlight(data.content, {
			language: language,
		});

		body.innerHTML = code.value;
	} else {
		const code = hljs.highlightAuto(data.content);

		language = code.language;

		body.innerHTML = code.value;
	}

	title.innerText += ` (${language})`;

	addEventListener("keydown", event => {
		const key = event.key.toLowerCase();

		if ((event.ctrlKey || event.metaKey) && key === "s") {
			event.preventDefault();

			const blob = new Blob([data.content], {
				type: "text/plain;charset=utf-8",
			});

			const el = document.createElement("a"),
				url = URL.createObjectURL(blob);

			el.download = data.name;
			el.href = url;

			el.click();

			setTimeout(() => URL.revokeObjectURL(url), 100);
		} else if ((event.ctrlKey || event.metaKey) && key === "a") {
			event.preventDefault();

			const sel = window.getSelection(),
				range = document.createRange();

			range.selectNodeContents(body);

			sel.removeAllRanges();
			sel.addRange(range);
		} else if (key === "escape") {
			window.close();
		}
	});
}

function addPrintLayout(content, layout) {
	const script = `<script>addEventListener("message", event => { if (event.data === "whiskr-print") window.print(); });<\/script>`,
		style = `<style>@page { size: ${layout}; margin: 12mm; }</style>`;

	if (!/<head[^>]*>/i.test(content)) {
		return script + style + content;
	}

	content = content.replace(/<head[^>]*>/i, match => match + script);

	return /<\/head\s*>/i.test(content) ? content.replace(/<\/head\s*>/i, `${style}</head>`) : content + style;
}

function guessLanguage(name) {
	const extensions = {
		js: "javascript",
		mjs: "javascript",
		cjs: "javascript",
		jsx: "javascript",
		ts: "typescript",
		tsx: "typescript",
		json: "json",
		jsonc: "json",
		html: "html",
		htm: "html",
		css: "css",
		scss: "scss",
		sass: "scss",
		less: "less",
		go: "go",
		php: "php",
		sh: "bash",
		bash: "bash",
		ps1: "powershell",
		bat: "dos",
		cmd: "dos",
		reg: "ini",
		env: "ini",
		properties: "properties",
		ini: "ini",
		conf: "ini",
		toml: "toml",
		yaml: "yaml",
		yml: "yaml",
		md: "markdown",
		txt: "text",
		csv: "csv",
		xml: "xml",
		svg: "xml",
		py: "python",
		rb: "ruby",
		rbs: "ruby",
		lua: "lua",
		pl: "perl",
		pm: "perl",
		java: "java",
		kt: "kotlin",
		kts: "kotlin",
		c: "c",
		h: "c",
		cc: "cpp",
		cpp: "cpp",
		cxx: "cpp",
		hh: "cpp",
		hpp: "cpp",
		hxx: "cpp",
		cs: "csharp",
		m: "objectivec",
		mm: "objectivec",
		rs: "rust",
		zig: "zig",
		asm: "x86asm",
		swift: "swift",
		sql: "sql",
		psq: "pgsql",
		psql: "pgsql",
		nginx: "nginx",
		proto: "protobuf",
		dockerfile: "dockerfile",
		ejs: "javascript",
		diff: "diff",
		patch: "diff",
		log: "text",
	};

	const test = name.toLowerCase().trim();

	if (test === "go.mod" || test === "go.sum") {
		return "go";
	}

	for (const ext in extensions) {
		if (test.endsWith(`.${ext}`)) {
			return extensions[ext];
		}
	}

	return null;
}

if (document.readyState === "complete" || document.readyState === "interactive") {
	init();
} else {
	document.addEventListener("DOMContentLoaded", init);
}
