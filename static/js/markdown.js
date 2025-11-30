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

	function parse(markdown) {
		const starts = (markdown.match(/^FILE\s+"([^"]+)"(?:\s+LINES\s+\d+)?\s*\r?\n<<CONTENT>>\s*$/gm) || []).length,
			ends = (markdown.match(/^<<END(?:ING)?>>$/gm) || []).length;

		if (starts !== ends) {
			markdown += "\n<<ENDING>>";
		}

		const files = [],
			table = {};

		markdown = markdown.replace(/^FILE\s+"([^"]+)"(?:\s+LINES\s+(\d+))?\s*\r?\n<<CONTENT>>\s*\r?\n([\s\S]*?)\r?\n<<END(ING)?>>$/gm, (_a, name, _b, content, ending) => {
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