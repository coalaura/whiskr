(() => {
	const timeouts = new WeakMap();

	marked.use({
		async: false,
		breaks: false,
		gfm: true,
		pedantic: false,

		walkTokens: (token) => {
			const { type, text } = token;

			if (type === "html") {
				token.text = token.text.replace(/&/g, "&amp;")
				token.text = token.text.replace(/</g, "&lt;")
				token.text = token.text.replace(/>/g, "&gt;")

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
	});

	document.body.addEventListener("click", (event) => {
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
			}, 1000),
		);
	});

	window.render = (markdown) => {
		return marked.parse(markdown);
	};
})();
