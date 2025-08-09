(() => {
	const timeouts = new WeakMap(),
		images = {};

	marked.use({
		async: false,
		breaks: false,
		gfm: true,
		pedantic: false,

		walkTokens: (token) => {
			const { type, lang, text } = token;

			if (type !== "code") {
				return;
			}

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

				return `<pre>${header}<code>${code.text}</code></pre>`;
			},

			image(image) {
				const { href } = image;

				const id = `i_${btoa(href).replace(/=/g, "")}`,
					style = prepareImage(id, href) || "";

				return `<div class="image ${id}" style="${style}"></div>`;
			},
		},
	});

	function prepareImage(id, href) {
		if (href in images) {
			return images[href];
		}

		images[href] = false;

		const image = new Image();

		image.addEventListener("load", () => {
			const style = `aspect-ratio:${image.naturalWidth}/${image.naturalHeight};width:${image.naturalWidth}px;background-image:url(${href})`;

			images[href] = style;

			document.querySelectorAll(`.image.${id}`).forEach((img) => {
				img.setAttribute("style", style);
			});

			window.dispatchEvent(new Event("image-loaded"));
		});

		image.addEventListener("error", () => {
			console.error(`Failed to load image: ${href}`);
		});

		image.src = href;

		return false;
	}

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
