(() => {
	const timeouts = new WeakMap(),
		scrollState = {
			el: null,
			startX: 0,
			scrollLeft: 0,
			pointerId: null,
			moved: false,
		};

	marked.use({
		async: false,
		breaks: false,
		gfm: true,
		pedantic: false,

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

	function formatBytes(bytes) {
		if (!+bytes) {
			return "0B";
		}

		const sizes = ["B", "kB", "MB", "GB", "TB"],
			i = Math.floor(Math.log(bytes) / Math.log(1000));

		const val = bytes / Math.pow(1000, i),
			dec = i === 0 ? 0 : val < 10 ? 2 : 1;

		return `${val.toFixed(dec)}${sizes[i]}`;
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

		const stack = [];
		// Regex matches: <tag ... >, </tag>, <tag ... />
		// Group 1: Slash (if closing)
		// Group 2: Tag Name
		// Group 3: Slash (if self-closing)
		const tagRegex = /<(\/)?([a-zA-Z0-9:_-]+)[^>]*?(\/)?>/g;
		let match;

		while ((match = tagRegex.exec(clean)) !== null) {
			const [fullMatch, isClosing, tagName, isSelfClosing] = match;

			// Ignore processing instructions (<?xml) or comments (though regex handles tags mostly)
			if (tagName.toLowerCase() === "?xml" || fullMatch.startsWith("<!--")) {
				continue;
			}

			if (isSelfClosing) {
				continue; // <path /> - no action needed
			}

			if (isClosing) {
				// </g> - Pop matching tag from stack
				// In a perfect world, we check if it matches the top of stack.
				// In a streaming world, we assume the LLM is mostly correct logic-wise.
				if (stack.length > 0 && stack[stack.length - 1] === tagName) {
					stack.pop();
				}
			} else {
				// <g> - Push to stack
				// Handle void tags if necessary (though rare in SVG besides image/path/rect which are usually self-closed)
				stack.push(tagName);
			}
		}

		// 3. Append missing closing tags in reverse order
		while (stack.length > 0) {
			const tag = stack.pop();
			clean += `</${tag}>`;
		}

		// 4. Encode to Base64
		// We use encodeURIComponent to handle Unicode characters correctly in btoa
		try {
			const b64 = btoa(encodeURIComponent(clean).replace(/%([0-9A-F]{2})/g, (match, p1) => String.fromCharCode("0x" + p1)));
			return `data:image/svg+xml;base64,${b64}`;
		} catch (err) {
			console.error("SVG generation error", err);
			return "";
		}
	}

	function parse(markdown) {
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

		const html = marked.parse(markdown).replace(/(?:<p>\s*)?§\|FILE\|(\d+)\|§(?:<\/p>\s*)?/g, (match, index) => {
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

	window.render = markdown => {
		return parse(markdown);
	};

	window.renderInline = markdown => {
		return marked.parseInline(markdown.trim());
	};

	window.stripMarkdown = markdown => {
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
	};
})();
