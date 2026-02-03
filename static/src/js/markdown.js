import "katex/dist/katex.min.css";

import hljs from "highlight.js";
import katex from "katex";
import { parse, parseInline, use } from "marked";

import { formatBytes } from "./lib.js";

const timeouts = new WeakMap(),
	scrollState = {
		el: null,
		startX: 0,
		scrollLeft: 0,
		pointerId: null,
		moved: false,
	};

const MathExtension = {
	name: "math",
	level: "inline",
	start: pSrc => pSrc.match(/\$|\\\(|\\\[/)?.index,
	tokenizer: pSrc => {
		const displayRule = /^(?:\$\$|\\\[)([\s\S]+?)(?:\$\$|\\\])/,
			displayMatch = displayRule.exec(pSrc);

		if (displayMatch) {
			return {
				type: "math",
				raw: displayMatch[0],
				text: displayMatch[1].trim(),
				displayMode: true,
			};
		}

		const inlineRule = /^(?:\$|\\\()([\s\S]+?)(?:\$|\\\))/,
			inlineMatch = inlineRule.exec(pSrc);

		if (inlineMatch) {
			return {
				type: "math",
				raw: inlineMatch[0],
				text: inlineMatch[1].trim(),
				displayMode: false,
			};
		}
	},
	renderer: pToken => {
		try {
			return katex.renderToString(pToken.text, {
				displayMode: pToken.displayMode,
				throwOnError: false,
			});
		} catch (pErr) {
			console.error("KaTeX error:", pErr);

			return pToken.raw;
		}
	},
};

use({
	async: false,
	breaks: false,
	gfm: true,
	pedantic: false,
	extensions: [MathExtension],

	walkTokens: token => {
		const { type, text } = token;

		if (type === "html") {
			token.text = escapeHtml(token.text);

			return;
		}

		if (type === "code") {
			if (text.trim().match(/^§\|FILE\|\d+\|§$/gm)) {
				token.type = "text";

				return;
			}

			const lang = token.lang || "plaintext";

			let code;

			if (lang && hljs.getLanguage(lang)) {
				code = hljs.highlight(text.trim(), {
					language: lang,
				});
			} else {
				code = hljs.highlightAuto(text.trim());
			}

			token.escaped = true;
			token.lang = code.language || "plaintext";
			token.text = code.value;
		}
	},

	renderer: {
		image: href => {
			const title = href.title ? ` title="${escapeHtml(href.title)}"` : "",
				alt = href.text ? ` alt="${escapeHtml(href.text)}"` : "";

			return `<span class="image-wrapper"><img src="${escapeHtml(href.href)}"${alt}${title} class="image" /></span>`;
		},

		code: code => {
			const header = `<div class="pre-header">${escapeHtml(code.lang)}</div>`;
			const button = `<button class="pre-copy" title="Copy code contents"></button>`;

			return `<pre class="l-${escapeHtml(code.lang)}">${header}${button}<code>${code.text}</code></pre>`;
		},

		link: link => `<a href="${link.href}" target="_blank">${escapeHtml(link.text || link.href)}</a>`,
	},

	hooks: {
		postprocess: html => {
			html = html.replace(/<table>/g, `<div class="table-wrapper"><table>`);
			html = html.replace(/<\/ ?table>/g, `</table></div>`);

			return html;
		},
	},
});

function generateID() {
	return `${Math.random().toString(36).slice(2)}${"0".repeat(8)}`.slice(0, 8);
}

function escapeHtml(text) {
	return text.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function fixStreamBuffer(markdown) {
	// fix the model forgetting to add the <<CONTENT>> line
	return markdown.replace(/(FILE\s+"[^"]+"(?:\s+LINES\s+[\d-]+)?)/g, (match, _header, offset, fullString) => {
		const rest = fullString.slice(offset + match.length);

		if (/^\s*<<CONTENT>>/.test(rest)) {
			return match;
		}

		if (/^\s*[\r\n]/.test(rest)) {
			return `${match}\n<<CONTENT>>`;
		}

		return match;
	});
}

function fixProgressiveSvg(raw) {
	const lastTagEnd = raw.lastIndexOf(">");

	if (lastTagEnd === -1) {
		return "";
	}

	let clean = raw.slice(0, lastTagEnd + 1);

	const stack = [],
		tagRegex = /<(\/)?([a-zA-Z0-9:_-]+)[^>]*?(\/)?>/g;

	let match;

	while ((match = tagRegex.exec(clean)) !== null) {
		const [fullMatch, isClosing, tagName, isSelfClosing] = match;

		if (tagName.toLowerCase() === "?xml" || fullMatch.startsWith("<!--")) {
			continue;
		}

		if (isSelfClosing) {
			continue;
		}

		if (isClosing) {
			if (stack.length > 0 && stack[stack.length - 1] === tagName) {
				stack.pop();
			}
		} else {
			stack.push(tagName);
		}
	}

	while (stack.length > 0) {
		clean += `</${stack.pop()}>`;
	}

	try {
		const b64 = btoa(encodeURIComponent(clean).replace(/%([0-9A-F]{2})/g, (_match, p1) => String.fromCharCode(`0x${p1}`)));

		return `data:image/svg+xml;base64,${b64}`;
	} catch (err) {
		console.error("SVG generation error", err);

		return "";
	}
}

function parseMd(markdown) {
	markdown = fixStreamBuffer(markdown);

	const starts = (markdown.match(/^ ?FILE\s+"([^"]+)"(?:\s+LINES\s+\d+(?:-\d+)?)?\s*\r?\n<<CONTENT>>\s*$/gm) || []).length,
		ends = (markdown.match(/^<<END(?:ING)?>>$/gm) || []).length;

	if (starts !== ends && starts > ends) {
		markdown += "\n<<ENDING>>";
	}

	const files = [],
		table = {};

	markdown = markdown.replace(/^ ?FILE\s+"([^"]+)"(?:\s+LINES\s+\d+(?:-\d+)?)?\s*\r?\n<<CONTENT>>\s*\r?\n([\s\S]*?)\r?\n<<END(ING)?>>$/gm, (_a, name, content, ending) => {
		const index = files.length,
			id = generateID();

		files.push({
			id: id,
			name: name,
			size: content.length,
			busy: !!ending,
		});

		table[id] = {
			name: name,
			content: content,
		};

		return `§|FILE|${index}|§`;
	});

	const html = parse(markdown).replace(/(?:<p>\s*)?§\|FILE\|(\d+)\|§(?:<\/p>\s*)?/g, (match, index) => {
		index = parseInt(index, 10);

		if (index < files.length) {
			const file = files[index],
				name = escapeHtml(file.name);

			if (file.name.endsWith(".svg")) {
				const svg = fixProgressiveSvg(table[file.id].content);

				return `<div class="inline-svg ${file.busy ? "busy" : ""}" data-id="${file.id}"><img class="image" src="${svg}" /><button class="download" title="Download file"></button></div>`;
			}

			return `<div class="inline-file ${file.busy ? "busy" : ""}" data-id="${file.id}"><div class="name" title="${name}">${name}</div><div class="size"><sup>${formatBytes(file.size)}</sup></div><button class="download" title="Download file"></button></div>`;
		}

		return match;
	});

	return {
		html: html,
		files: table,
	};
}

addEventListener("click", event => {
	const button = event.target;

	if (!button.classList.contains("pre-copy")) {
		return;
	}

	const pre = button.closest("pre"),
		code = pre?.querySelector("code");

	if (!code) {
		return;
	}

	clearTimeout(timeouts.get(pre));

	navigator.clipboard.writeText(code.textContent.trim());

	button.classList.add("copied");

	timeouts.set(
		pre,
		setTimeout(() => {
			button.classList.remove("copied");
		}, 1000)
	);
});

addEventListener("pointerover", event => {
	if (event.pointerType !== "mouse") {
		return;
	}

	const el = event.target.closest(".table-wrapper");

	if (!el) {
		return;
	}

	el.classList.toggle("overflowing", el.scrollWidth - el.clientWidth > 1);
});

addEventListener("pointerdown", event => {
	if (event.button !== 0) {
		return;
	}

	const el = event.target.closest(".table-wrapper");

	if (!el || !el.classList.contains("overflowing")) {
		return;
	}

	scrollState.el = el;
	scrollState.pointerId = event.pointerId;
	scrollState.startX = event.clientX;
	scrollState.scrollLeft = el.scrollLeft;
	scrollState.moved = false;

	el.classList.add("dragging");
	el.setPointerCapture?.(event.pointerId);

	event.preventDefault();
});

addEventListener("pointermove", event => {
	if (!scrollState.el || event.pointerId !== scrollState.pointerId) {
		return;
	}

	const dx = event.clientX - scrollState.startX;

	if (Math.abs(dx) > 3) {
		scrollState.moved = true;
	}

	scrollState.el.scrollLeft = scrollState.scrollLeft - dx;
});

function endScroll(event) {
	if (!scrollState.el || (event && event.pointerId !== scrollState.pointerId)) {
		return;
	}

	scrollState.el.classList.remove("dragging");
	scrollState.el.releasePointerCapture?.(scrollState.pointerId);
	scrollState.el = null;
	scrollState.pointerId = null;
}

addEventListener("pointerup", endScroll);
addEventListener("pointercancel", endScroll);

export function render(markdown) {
	return parseMd(markdown);
}

export function renderInline(markdown) {
	return parseInline(markdown.trim());
}

export function stripMarkdown(markdown) {
	return (
		markdown
			// Remove headings
			.replace(/^#+\s*/gm, "")
			// Remove links, keeping the link text
			.replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
			// Remove bold/italics, keeping the text
			.replace(/(\*\*|__|\*|_|~~|`)(.*?)\1/g, "$2")
			// Remove images
			.replace(/!\[[^\]]*\]\([^)]*\)/g, "")
			// Remove horizontal rules
			.replace(/^-{3,}\s*$/gm, "")
			// Remove list markers
			.replace(/^\s*([*-]|\d+\.)\s+/gm, "")
			// Collapse multiple newlines into one
			.replace(/\n{2,}/g, "\n")
			.trim()
	);
}
