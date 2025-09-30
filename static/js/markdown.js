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
				token.text = token.text.replace(/&/g, "&amp;");
				token.text = token.text.replace(/</g, "&lt;");
				token.text = token.text.replace(/>/g, "&gt;");

				return;
			} else if (type !== "code") {
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
		},

		renderer: {
			code(code) {
				const header = `<div class="pre-header">${escapeHtml(code.lang)}<button class="pre-copy" title="Copy code contents"></button></div>`;

				return `<pre class="l-${escapeHtml(code.lang)}">${header}<code>${code.text}</code></pre>`;
			},

			link(link) {
				return `<a href="${link.href}" target="_blank">${escapeHtml(link.text || link.href)}</a>`;
			},
		},

		hooks: {
			postprocess: html => {
				html = html.replace(/<table>/g, `<div class="table-wrapper"><table>`);
				html = html.replace(/<\/ ?table>/g, `</table></div>`);

				return html;
			},
		},
	});

	addEventListener("click", event => {
		const button = event.target,
			header = button.closest(".pre-header"),
			pre = header?.closest("pre"),
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
		return marked.parse(markdown);
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
				.replace(/\[([^\]]+)\]\([^\)]+\)/g, "$1")
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
