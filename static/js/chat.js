(() => {
	const ChunkType = {
		0: "start",
		1: "id",
		2: "reason",
		3: "text",
		4: "image",
		5: "tool",
		6: "error",
		7: "end",
	};

	const $version = document.getElementById("version"),
		$total = document.getElementById("total"),
		$title = document.getElementById("title"),
		$titleRefresh = document.getElementById("title-refresh"),
		$titleText = document.getElementById("title-text"),
		$messages = document.getElementById("messages"),
		$chat = document.getElementById("chat"),
		$message = document.getElementById("message"),
		$top = document.getElementById("top"),
		$bottom = document.getElementById("bottom"),
		$resizeBar = document.getElementById("resize-bar"),
		$attachments = document.getElementById("attachments"),
		$role = document.getElementById("role"),
		$model = document.getElementById("model"),
		$prompt = document.getElementById("prompt"),
		$temperature = document.getElementById("temperature"),
		$iterations = document.getElementById("iterations"),
		$reasoningEffort = document.getElementById("reasoning-effort"),
		$reasoningTokens = document.getElementById("reasoning-tokens"),
		$json = document.getElementById("json"),
		$search = document.getElementById("search"),
		$upload = document.getElementById("upload"),
		$add = document.getElementById("add"),
		$send = document.getElementById("send"),
		$scrolling = document.getElementById("scrolling"),
		$export = document.getElementById("export"),
		$import = document.getElementById("import"),
		$clear = document.getElementById("clear"),
		$authentication = document.getElementById("authentication"),
		$authError = document.getElementById("auth-error"),
		$username = document.getElementById("username"),
		$password = document.getElementById("password"),
		$login = document.getElementById("login");

	const nearBottom = 22,
		timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";

	let platform = "";

	detectPlatform().then(result => {
		platform = result;

		console.info(`Detected platform: ${platform}`);
	});

	const messages = [],
		models = {},
		modelList = [],
		promptList = [];

	let autoScrolling = false,
		followTail = true,
		awaitingScroll = false,
		jsonMode = false,
		searchTool = false,
		chatTitle = false;

	let searchAvailable = false,
		activeMessage = null,
		isResizing = false,
		scrollResize = false,
		isUploading = false,
		totalCost = 0;

	function updateTotalCost() {
		storeValue("total-cost", totalCost);

		$total.textContent = formatMoney(totalCost);
	}

	function updateTitle() {
		const title = chatTitle || (messages.length ? "New Chat" : "");

		$title.classList.toggle("hidden", !messages.length);

		$titleText.textContent = title;

		document.title = `whiskr${chatTitle ? ` - ${chatTitle}` : ""}`;

		storeValue("title", chatTitle);
	}

	function distanceFromBottom() {
		return $messages.scrollHeight - ($messages.scrollTop + $messages.clientHeight);
	}

	function updateScrollButton() {
		const bottom = distanceFromBottom();

		$top.classList.toggle("hidden", $messages.scrollTop < 80);
		$bottom.classList.toggle("hidden", bottom < 80);
	}

	function setFollowTail(follow) {
		followTail = follow;

		$scrolling.classList.toggle("not-following", !followTail);
	}

	function scroll(force = false, instant = false) {
		if (awaitingScroll || !(followTail || force)) {
			updateScrollButton();

			return;
		}

		awaitingScroll = true;

		requestAnimationFrame(() => {
			awaitingScroll = false;

			if (!followTail && !force) {
				return;
			}

			$messages.scroll({
				top: $messages.scrollHeight,
				behavior: instant ? "instant" : "smooth",
			});
		});
	}

	function preloadIcons(icons) {
		for (const icon of icons) {
			new Image().src = `/css/icons/${icon}`;
		}
	}

	function mark(index) {
		for (let x = 0; x < messages.length; x++) {
			messages[x].mark(Number.isInteger(index) && x >= index);
		}
	}

	class Message {
		#destroyed = false;

		#id;
		#role;
		#reasoning;
		#text;
		#images = [];
		#files = [];

		#tool;
		#tags = [];
		#statistics;
		#error = false;

		#editing = false;
		#state = false;
		#loading = false;

		#_diff;
		#pending = {};
		#patching = {};

		#_message;
		#_tags;
		#_files;
		#_reasoning;
		#_text;
		#_edit;
		#_images;
		#_tool;
		#_statistics;

		constructor(role, reasoning, text, tool = false, files = [], images = [], tags = [], collapsed = false) {
			this.#id = uid();
			this.#role = role;
			this.#reasoning = reasoning || "";
			this.#text = text || "";

			this.#_diff = document.createElement("div");

			this.#build(collapsed);
			this.#render();

			if (tool?.name) {
				this.setTool(tool);
			}

			for (const file of files) {
				this.addFile(file);
			}

			for (const image of images) {
				this.addImage(image);
			}

			for (const tag of tags) {
				this.addTag(tag);
			}

			messages.push(this);

			this.#save();
		}

		#build(collapsed) {
			// main message div
			this.#_message = make("div", "message", this.#role, collapsed ? "collapsed" : "");

			// message role (wrapper)
			const _wrapper = make("div", "role", this.#role);

			this.#_message.appendChild(_wrapper);

			// message role
			const _role = make("div");

			_role.textContent = this.#role;

			_wrapper.appendChild(_role);

			// message tags
			this.#_tags = make("div", "tags");

			_wrapper.appendChild(this.#_tags);

			const _body = make("div", "body");

			this.#_message.appendChild(_body);

			// loader
			const _loader = make("div", "loader");

			_loader.innerHTML = "<span></span>".repeat(3);

			_body.appendChild(_loader);

			// message files
			this.#_files = make("div", "files");

			_body.appendChild(this.#_files);

			// message reasoning (wrapper)
			const _reasoning = make("div", "reasoning");

			_body.appendChild(_reasoning);

			// message reasoning (toggle)
			const _toggle = make("button", "toggle");

			_toggle.textContent = "Reasoning";

			_reasoning.appendChild(_toggle);

			_toggle.addEventListener("click", () => {
				let delta = this.#updateReasoningHeight() + 16; // margin

				if (!_reasoning.classList.toggle("expanded")) {
					delta = -delta;
				}

				setFollowTail(distanceFromBottom() + delta <= nearBottom);

				updateScrollButton();
			});

			// message reasoning (content)
			this.#_reasoning = make("div", "reasoning-text", "markdown");

			_reasoning.appendChild(this.#_reasoning);

			// message content
			this.#_text = make("div", "text", "markdown");

			_body.appendChild(this.#_text);

			// message edit textarea
			this.#_edit = make("textarea", "text");

			_body.appendChild(this.#_edit);

			this.#_edit.addEventListener("keydown", event => {
				if (event.ctrlKey && event.key === "Enter") {
					this.toggleEdit();
				} else if (event.key === "Escape") {
					this.#_edit.value = this.#text;

					this.toggleEdit();
				}
			});

			this.#_edit.addEventListener("input", () => {
				this.updateEditHeight();
			});

			// message images
			this.#_images = make("div", "images");

			_body.appendChild(this.#_images);

			// message tool
			this.#_tool = make("div", "tool");

			_body.appendChild(this.#_tool);

			// tool call
			const _call = make("div", "call");

			this.#_tool.appendChild(_call);

			_call.addEventListener("click", () => {
				let delta = this.#updateToolHeight() + 16; // margin

				if (!this.#_tool.classList.toggle("expanded")) {
					delta = -delta;
				}

				setFollowTail(distanceFromBottom() + delta <= nearBottom);

				updateScrollButton();
			});

			// tool call name
			const _callName = make("div", "name");

			_call.appendChild(_callName);

			// tool call arguments
			const _callArguments = make("div", "arguments");

			_call.appendChild(_callArguments);

			// tool call cost
			const _callCost = make("div", "cost");

			_callCost.title = "Cost of this tool call";

			this.#_tool.appendChild(_callCost);

			// tool call result
			const _callResult = make("div", "result", "markdown");

			this.#_tool.appendChild(_callResult);

			// message options
			const _opts = make("div", "options");

			this.#_message.appendChild(_opts);

			// collapse option
			const _optCollapse = make("button", "collapse");

			_optCollapse.title = "Collapse/Expand message";

			_opts.appendChild(_optCollapse);

			_optCollapse.addEventListener("click", () => {
				this.#_message.classList.toggle("collapsed");

				updateScrollButton();

				setFollowTail(distanceFromBottom() <= nearBottom);

				this.#save();
			});

			// copy option
			const _optCopy = make("button", "copy");

			_optCopy.title = "Copy message content";

			_opts.appendChild(_optCopy);

			let timeout;

			_optCopy.addEventListener("click", () => {
				this.stopEdit();

				clearTimeout(timeout);

				navigator.clipboard.writeText(this.#text);

				_optCopy.classList.add("copied");

				timeout = setTimeout(() => {
					_optCopy.classList.remove("copied");
				}, 1000);
			});

			// retry option
			const _assistant = this.#role === "assistant",
				_retryLabel = _assistant ? "Delete message and messages after this one and try again" : "Delete messages after this one and try again";

			const _optRetry = make("button", "retry");

			_optRetry.title = _retryLabel;

			_opts.appendChild(_optRetry);

			_optRetry.addEventListener("mouseenter", () => {
				const index = this.index(!_assistant ? 1 : 0);

				mark(index);
			});

			_optRetry.addEventListener("mouseleave", () => {
				mark(false);
			});

			_optRetry.addEventListener("click", () => {
				const index = this.index(!_assistant ? 1 : 0);

				if (index === false) {
					return;
				}

				this.stopEdit();

				while (messages.length > index) {
					messages[messages.length - 1].delete();
				}

				mark(false);

				generate(false, true);
			});

			// edit option
			const _optEdit = make("button", "edit");

			_optEdit.title = "Edit message content";

			_opts.appendChild(_optEdit);

			_optEdit.addEventListener("click", () => {
				this.toggleEdit();
			});

			// delete option
			const _optDelete = make("button", "delete");

			_optDelete.title = "Delete message";

			_opts.appendChild(_optDelete);

			_optDelete.addEventListener("click", () => {
				this.delete();
			});

			// statistics
			this.#_statistics = make("div", "statistics");

			this.#_message.appendChild(this.#_statistics);

			// add to dom
			$messages.appendChild(this.#_message);

			scroll();
		}

		#handleImages(element) {
			element.querySelectorAll("img:not(.image)").forEach(img => {
				img.classList.add("image");

				img.addEventListener("load", () => {
					scroll();
				});
			});
		}

		#updateReasoningHeight() {
			const height = this.#_reasoning.scrollHeight;

			this.#_reasoning.parentNode.style.setProperty("--height", `${height}px`);

			return height;
		}

		#updateToolHeight() {
			const result = this.#_tool.querySelector(".result"),
				height = result.scrollHeight;

			this.#_tool.style.setProperty("--height", `${height}px`);

			return height;
		}

		#morph(from, to) {
			morphdom(from, to, {
				childrenOnly: true,
				onBeforeElUpdated: (fromEl, toEl) => {
					return !fromEl.isEqualNode || !fromEl.isEqualNode(toEl);
				},
			});
		}

		#patch(name, element, md, after = false) {
			if (!element.firstChild) {
				element.innerHTML = render(md);

				this.#handleImages(element);

				after?.();

				return;
			}

			this.#pending[name] = md;

			if (this.#patching[name]) {
				return;
			}

			this.#patching[name] = true;

			schedule(() => {
				const html = render(this.#pending[name]);

				this.#patching[name] = false;

				this.#_diff.innerHTML = html;

				this.#morph(element, this.#_diff);

				this.#_diff.innerHTML = "";

				this.#handleImages(element);

				after?.();
			});
		}

		#render(only = false, noScroll = false) {
			if (!only || only === "tags") {
				const tags = this.#tags.map(tag => `<div class="tag-${tag}" title="${tag}"></div>`);

				this.#_tags.innerHTML = tags.join("");

				this.#_message.classList.toggle("has-tags", this.#tags.length > 0);
			}

			if (!only || only === "images") {
				for (let x = 0; x < this.#images.length; x++) {
					if (this.#_images.querySelector(`.i-${x}`)) {
						continue;
					}

					const image = this.#images[x],
						blob = dataBlob(image),
						url = URL.createObjectURL(blob);

					const _link = make("a", "image", `i-${x}`);

					_link.download = `image-${x + 1}`;
					_link.target = "_blank";
					_link.href = url;

					this.#_images.appendChild(_link);

					const _image = make("img");

					_image.src = url;

					_link.appendChild(_image);
				}

				this.#_message.classList.toggle("has-images", !!this.#images.length);

				noScroll || scroll();

				updateScrollButton();
			}

			if (!only || only === "tool") {
				if (this.#tool) {
					const { name, args, result, cost, invalid } = this.#tool;

					const _name = this.#_tool.querySelector(".name"),
						_arguments = this.#_tool.querySelector(".arguments"),
						_cost = this.#_tool.querySelector(".cost"),
						_result = this.#_tool.querySelector(".result");

					_name.title = `Show ${name} call result`;
					_name.textContent = name;

					_arguments.title = args;
					_arguments.textContent = args;

					_cost.textContent = cost ? `${formatMoney(cost)}` : "";

					_result.classList.toggle("error", result?.startsWith("error: "));
					_result.innerHTML = render(result ? wrapJSON(result) : "*processing*");

					this.#_tool.classList.toggle("invalid", !!invalid);

					this.#_tool.setAttribute("data-tool", name);
				} else {
					this.#_tool.removeAttribute("data-tool");
				}

				this.#_message.classList.toggle("has-tool", !!this.#tool);

				this.#updateToolHeight();

				noScroll || scroll();

				updateScrollButton();
			}

			if (!only || only === "statistics") {
				let html = "";

				if (this.#statistics) {
					const { provider, model, ttft, time, input, output, cost } = this.#statistics;

					const tps = output / (time / 1000);

					html = [
						provider ? `<div class="provider">${provider} (${model.split("/").pop()})</div>` : "",
						`<div class="ttft">${formatMilliseconds(ttft)}</div>`,
						`<div class="tps">${fixed(tps, 2)} t/s</div>`,
						`<div class="tokens">
							<div class="input">${input}</div>
							+
							<div class="output">${output}</div>
							=
							<div class="total">${input + output}t</div>
						</div>`,
						`<div class="cost">${formatMoney(cost)}</div>`,
					].join("");
				}

				this.#_statistics.innerHTML = html;

				this.#_message.classList.toggle("has-statistics", !!html);
			}

			if (this.#error) {
				return;
			}

			if (!only || only === "reasoning") {
				this.#patch("reasoning", this.#_reasoning, this.#reasoning, () => {
					this.#updateReasoningHeight();

					noScroll || scroll();

					updateScrollButton();
				});

				this.#_message.classList.toggle("has-reasoning", !!this.#reasoning);
			}

			if (!only || only === "text") {
				let text = this.#text;

				if (text && this.#tags.includes("json")) {
					text = `\`\`\`json\n${text}\n\`\`\``;
				}

				this.#patch("text", this.#_text, text, () => {
					noScroll || scroll();

					updateScrollButton();
				});

				this.#_message.classList.toggle("has-text", !!this.#text);
			}
		}

		#save() {
			storeValue("messages", messages.map(message => message.getData(true)).filter(Boolean));
		}

		isUser() {
			return this.#role === "user";
		}

		index(offset = 0) {
			const index = messages.findIndex(message => message.#id === this.#id);

			if (index === -1) {
				return false;
			}

			return index + offset;
		}

		mark(state = false) {
			this.#_message.classList.toggle("marked", state);
		}

		getData(full = false) {
			const data = {
				role: this.#role,
				text: this.#text,
			};

			if (this.#files.length) {
				data.files = this.#files.map(file => ({
					name: file.name,
					content: file.content,
					tokens: file.tokens,
				}));
			}

			if (this.#tool) {
				data.tool = this.#tool;
			}

			if (this.#reasoning && full) {
				data.reasoning = this.#reasoning;
			}

			if (this.#images.length && full) {
				data.images = this.#images;
			}

			if (this.#error && full) {
				data.error = this.#error;
			}

			if (this.#tags.length && full) {
				data.tags = this.#tags;
			}

			if (this.#statistics && full) {
				data.statistics = this.#statistics;
			}

			if (this.#_message.classList.contains("collapsed") && full) {
				data.collapsed = true;
			}

			if (!data.error && !data.images?.length && !data.files?.length && !data.reasoning && !data.text && !data.tool) {
				return false;
			}

			return data;
		}

		setStatistics(statistics) {
			this.#statistics = statistics;

			this.#render("statistics");
			this.#save();
		}

		async loadGenerationData(generationID, retrying = false) {
			if (!generationID || this.#destroyed) {
				return;
			}

			try {
				const response = await fetch(`/-/stats/${generationID}`),
					data = await response.json();

				if (!data || data.error) {
					throw new Error(data?.error || response.statusText);
				}

				this.setStatistics(data);

				totalCost += data.cost;

				updateTotalCost();
			} catch (err) {
				console.error(err);

				if (!retrying && err.message.includes("not found")) {
					setTimeout(this.loadGenerationData.bind(this), 1500, generationID, true);
				}
			}
		}

		addTag(tag) {
			if (this.#tags.includes(tag)) {
				return;
			}

			this.#tags.push(tag);

			this.#render("tags");

			if (tag === "json") {
				this.#render("text");
			}

			this.#save();
		}

		addFile(file) {
			this.#files.push(file);

			this.#_files.appendChild(
				buildFileElement(file, el => {
					const index = this.#files.findIndex(attachment => attachment.id === file.id);

					if (index === -1) {
						return;
					}

					this.#files.splice(index, 1);

					el.remove();

					this.#_files.classList.toggle("has-files", !!this.#files.length);
					this.#_message.classList.toggle("has-files", !!this.#files.length);

					this.#save();
				})
			);

			this.#_files.classList.add("has-files");
			this.#_message.classList.add("has-files");

			this.#save();
		}

		setLoading(loading) {
			if (this.#loading === loading) {
				return;
			}

			this.#loading = loading;

			this.#_message.classList.toggle("loading", this.#loading);
		}

		setState(state) {
			if (this.#state === state) {
				return;
			}

			if (this.#state) {
				this.#_message.classList.remove(this.#state);
			}

			if (state) {
				this.#_message.classList.add(state);
			} else {
				if (this.#tool && !this.#tool.result) {
					this.#tool.result = "failed to run tool";

					this.#render("tool");
				}
			}

			this.#state = state;
		}

		setTool(tool) {
			this.#tool = tool;

			this.#render("tool");
			this.#save();
		}

		addImage(image) {
			this.#images.push(image);

			this.#render("images");
			this.#save();
		}

		addReasoning(chunk) {
			this.#reasoning += chunk;

			this.#render("reasoning");
			this.#save();
		}

		addText(text) {
			this.#text += text;

			this.#render("text");
			this.#save();
		}

		setError(error) {
			this.#error = error || "Something went wrong";

			this.#_message.classList.add("errored", "has-text");

			const _err = make("div", "error");

			_err.innerHTML = renderInline(this.#error);

			this.#_text.appendChild(_err);

			this.#save();
		}

		stopEdit() {
			if (!this.#editing) {
				return;
			}

			this.toggleEdit();
		}

		updateEditHeight() {
			this.#_edit.style.height = "";
			this.#_edit.style.height = `${Math.max(100, this.#_edit.scrollHeight + 2)}px`;
		}

		toggleEdit() {
			this.#editing = !this.#editing;

			if (this.#editing) {
				activeMessage = this;

				this.#_edit.value = this.#text;

				this.setState("editing");

				this.updateEditHeight();

				this.#_edit.focus();
			} else {
				activeMessage = null;

				this.#text = this.#_edit.value;

				this.setState(false);

				this.#render(false, true);
				this.#save();
			}

			setFollowTail(distanceFromBottom() <= nearBottom);

			updateScrollButton();
		}

		delete() {
			this.#destroyed = true;

			const index = messages.findIndex(msg => msg.#id === this.#id);

			if (index === -1) {
				return;
			}

			this.#_message.remove();

			messages.splice(index, 1);

			setFollowTail(distanceFromBottom() <= nearBottom);

			this.#save();

			$messages.dispatchEvent(new Event("scroll"));
		}
	}

	async function json(url) {
		try {
			const response = await fetch(url);

			if (!response.ok) {
				throw new Error(response.statusText);
			}

			return await response.json();
		} catch (err) {
			console.error(err);

			return false;
		}
	}

	async function stream(url, options, callback) {
		let aborted;

		try {
			const response = await fetch(url, options);

			if (!response.ok) {
				const err = await response.json();

				if (err?.error === "unauthorized") {
					showLogin();
				}

				throw new Error(err?.error || response.statusText);
			}

			const reader = response.body.getReader();

			let buffer = new Uint8Array();

			while (true) {
				const { value, done } = await reader.read();

				if (done) {
					break;
				}

				const read = new Uint8Array(buffer.length + value.length);

				read.set(buffer);
				read.set(value, buffer.length);

				buffer = read;

				while (buffer.length >= 5) {
					const type = ChunkType[buffer[0]],
						length = buffer[1] | (buffer[2] << 8) | (buffer[3] << 16) | (buffer[4] << 24);

					if (!type) {
						console.warn("bad chunk type", type);

						buffer = buffer.slice(5 + length);

						continue;
					}

					if (buffer.length < 5 + length) {
						break;
					}

					let data;

					if (length > 0) {
						const packed = buffer.slice(5, 5 + length);

						try {
							data = msgpackr.unpack(packed);
						} catch (err) {
							console.warn("bad chunk data", packed);
							console.warn(err);
						}
					}

					buffer = buffer.slice(5 + length);

					callback({
						type: type,
						data: data,
					});
				}
			}
		} catch (err) {
			if (err.name === "AbortError") {
				aborted = true;

				return;
			}

			callback({
				type: "error",
				data: err.message,
			});
		} finally {
			callback(aborted ? "aborted" : "done");
		}
	}

	let chatController;

	function generate(cancel = false, noPush = false) {
		if (chatController) {
			chatController.abort();

			if (cancel) {
				$chat.classList.remove("completing");

				return;
			}
		}

		if (autoScrolling) {
			setFollowTail(true);
		}

		let temperature = parseFloat($temperature.value);

		if (Number.isNaN(temperature) || temperature < 0 || temperature > 2) {
			temperature = 0.85;

			$temperature.value = temperature;
			$temperature.classList.remove("invalid");
		}

		let iterations = parseInt($iterations.value, 10);

		if (Number.isNaN(iterations) || iterations < 1 || iterations > 50) {
			iterations = 3;

			$iterations.value = iterations;
			$iterations.classList.remove("invalid");
		}

		const effort = $reasoningEffort.value;

		let tokens = parseInt($reasoningTokens.value, 10);

		if (!effort && (Number.isNaN(tokens) || tokens <= 0 || tokens > 1024 * 1024)) {
			tokens = 1024;

			$reasoningTokens.value = tokens;
			$reasoningTokens.classList.remove("invalid");
		}

		if (!noPush) {
			pushMessage();
		}

		chatController = new AbortController();

		$chat.classList.add("completing");

		const body = {
			prompt: $prompt.value,
			model: $model.value,
			temperature: temperature,
			iterations: iterations,
			tools: {
				json: jsonMode,
				search: searchTool,
			},
			reasoning: {
				effort: effort,
				tokens: tokens || 0,
			},
			metadata: {
				timezone: timezone,
				platform: platform,
			},
			messages: messages.map(message => message.getData()).filter(Boolean),
		};

		let message, generationID, timeout;

		function stopLoadingTimeout() {
			clearTimeout(timeout);

			message?.setLoading(false);
		}

		function startLoadingTimeout() {
			clearTimeout(timeout);

			timeout = setTimeout(() => {
				message?.setLoading(true);
			}, 1500);
		}

		function finish() {
			if (!message) {
				return;
			}

			const msg = message,
				genID = generationID;

			msg.setState(false);

			setTimeout(() => {
				msg.loadGenerationData(genID);
			}, 1000);

			message = null;
			generationID = null;
		}

		function start() {
			message = new Message("assistant", "", "");

			message.setState("waiting");

			if (jsonMode) {
				message.addTag("json");
			}

			if (searchTool) {
				message.addTag("search");
			}
		}

		start();

		stream(
			"/-/chat",
			{
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				body: JSON.stringify(body),
				signal: chatController.signal,
			},
			chunk => {
				stopLoadingTimeout();

				if (chunk === "aborted" || chunk === "done") {
					chatController = null;

					finish();

					if (chunk === "done") {
						$chat.classList.remove("completing");

						if (!chatTitle && !titleController) {
							refreshTitle();
						}
					}

					return;
				}

				if (!message && chunk.type !== "end") {
					start();
				}

				switch (chunk.type) {
					case "end":
						finish();

						break;
					case "id":
						generationID = chunk.data;

						break;
					case "tool":
						message.setState("tooling");
						message.setTool(chunk.data);

						if (chunk.data?.done) {
							totalCost += chunk.data.cost || 0;

							finish();
						} else {
							return; // prevent loading bar
						}

						break;
					case "image":
						message.addImage(chunk.data);

						break;
					case "reason":
						message.setState("reasoning");
						message.addReasoning(chunk.data);

						break;
					case "text":
						message.setState("receiving");
						message.addText(chunk.data);

						break;
					case "error":
						message.setError(chunk.data);

						break;
				}

				startLoadingTimeout();
			}
		);
	}

	let titleController;

	async function refreshTitle() {
		if (titleController) {
			titleController.abort();
		}

		titleController = new AbortController();

		const body = {
			title: chatTitle || null,
			messages: messages.map(message => message.getData()).filter(Boolean),
		};

		if (!body.messages.length) {
			chatTitle = false;

			updateTitle();

			return;
		}

		$title.classList.add("refreshing");

		try {
			const response = await fetch("/-/title", {
					method: "POST",
					headers: {
						"Content-Type": "application/json",
					},
					body: JSON.stringify(body),
					signal: titleController.signal,
				}),
				result = await response.json();

			if (result.cost) {
				totalCost += result.cost;

				updateTotalCost();
			}

			if (!response.ok || !result?.title) {
				throw new Error(result?.error || response.statusText);
			}

			chatTitle = result.title;
		} catch (err) {
			if (err.name === "AbortError") {
				return;
			}

			notify(err);
		}

		titleController = null;

		updateTitle();

		$title.classList.remove("refreshing");
	}

	async function login() {
		const username = $username.value.trim(),
			password = $password.value.trim();

		if (!username || !password) {
			throw new Error("missing username or password");
		}

		const data = await fetch("/-/auth", {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify({
				username: username,
				password: password,
			}),
		}).then(response => response.json());

		if (!data?.authenticated) {
			throw new Error(data.error || "authentication failed");
		}
	}

	function showLogin() {
		$password.value = "";

		$authentication.classList.add("open");
	}

	function initFloaters() {
		const $floaters = document.getElementById("floaters"),
			colors = ["#8aadf4", "#c6a0f6", "#8bd5ca", "#91d7e3", "#b7bdf8"],
			count = Math.floor((window.outerHeight * window.outerWidth) / 98000);

		function rand(a, b, rnd = false) {
			const num = Math.random() * (b - a) + a;

			if (rnd) {
				return Math.floor(num);
			}

			return num;
		}

		function place(el, init = false) {
			el.style.setProperty("--x", `${rand(0, 100).toFixed(4)}vw`);
			el.style.setProperty("--y", `${rand(0, 100).toFixed(4)}vh`);

			if (init) {
				return;
			}

			const time = rand(140, 160);

			el.style.setProperty("--time", `${time.toFixed(2)}s`);

			setTimeout(() => {
				place(el);
			}, time * 1000);
		}

		for (let i = 0; i < count; i++) {
			const el = document.createElement("div");

			el.className = "floater";

			el.style.setProperty("--size", `${rand(2, 4, true)}px`);
			el.style.setProperty("--color", colors[rand(0, colors.length, true)]);

			$floaters.appendChild(el);

			place(el, true);

			setTimeout(
				() => {
					place(el);
				},
				rand(0, 1000, true)
			);
		}
	}

	async function loadData() {
		const [_, data] = await Promise.all([connectDB(), json("/-/data")]);

		if (!data) {
			notify("Failed to load data.", true);

			return;
		}

		// start icon preload
		preloadIcons(data.icons);

		// render total cost
		totalCost = loadValue("total-cost", 0);

		updateTotalCost();

		// render version
		if (data.version === "dev") {
			$version.remove();
		} else {
			$version.innerHTML = `<a href="https://github.com/coalaura/whiskr" target="_blank">whiskr</a> <a href="https://github.com/coalaura/whiskr/releases/tag/${data.version}" target="_blank">${data.version}</a>`;
		}

		// update search availability
		searchAvailable = data.config.search;

		// initialize floaters (unless disabled)
		if (!data.config.motion) {
			initFloaters();
		}

		// show login modal
		if (data.config.authentication && !data.authenticated) {
			$authentication.classList.add("open");
		}

		// render models
		fillSelect($model, data.models, (el, model) => {
			const separator = "─".repeat(24);

			el.title = [
				model.name,
				separator,
				`Created:\t\t${formatTimestamp(model.created)}`,
				`Pricing/1M:\t${formatMoney(model.pricing.input)} In | ${formatMoney(model.pricing.output)} Out`,
				model.pricing.image ? `Images:\t\t${formatMoney(model.pricing.image)} each` : null,
				separator,
				stripMarkdown(model.description),
			]
				.filter(Boolean)
				.join("\n");

			el.value = model.id;
			el.textContent = model.name;

			el.dataset.tags = (model.tags || []).join(",");

			models[model.id] = model;
			modelList.push(model);
		});

		dropdown($model, 4);

		// render prompts
		data.prompts.unshift({
			key: "",
			name: "No Prompt",
		});

		fillSelect($prompt, data.prompts, (el, prompt) => {
			el.value = prompt.key;
			el.textContent = prompt.name;

			promptList.push(prompt);
		});

		dropdown($prompt);
	}

	function clearMessages() {
		while (messages.length) {
			messages[0].delete();
		}
	}

	function restore() {
		const resizedHeight = loadValue("resized");

		if (resizedHeight) {
			$chat.style.height = `${resizedHeight}px`;
		}

		$message.value = loadValue("message", "");
		$role.value = loadValue("role", "user");
		$model.value = loadValue("model", modelList.length ? modelList[0].id : "");
		$prompt.value = loadValue("prompt", promptList.length ? promptList[0].key : "");
		$temperature.value = loadValue("temperature", 0.85);
		$iterations.value = loadValue("iterations", 3);
		$reasoningEffort.value = loadValue("reasoning-effort", "medium");
		$reasoningTokens.value = loadValue("reasoning-tokens", 1024);

		const files = loadValue("attachments", []);

		for (const file of files) {
			pushAttachment(file);
		}

		if (loadValue("json")) {
			$json.click();
		}

		if (loadValue("search")) {
			$search.click();
		}

		if (loadValue("scrolling")) {
			$scrolling.click();
		}

		loadValue("messages", []).forEach(message => {
			const obj = new Message(message.role, message.reasoning, message.text, message.tool, message.files || [], message.images || [], message.tags || [], message.collapsed);

			if (message.statistics) {
				obj.setStatistics(message.statistics);
			}

			if (message.error) {
				obj.setError(message.error);
			}
		});

		chatTitle = loadValue("title");

		updateTitle();

		requestAnimationFrame(() => {
			$messages.scrollTop = $messages.scrollHeight;
		});
	}

	async function resolveTokenCount(str) {
		try {
			const response = await fetch("/-/tokenize", {
					method: "POST",
					headers: {
						"Content-Type": "application/json",
					},
					body: JSON.stringify({
						string: str,
					}),
				}),
				data = await response.json();

			if (!response.ok) {
				throw new Error(data?.error || response.statusText);
			}

			return data.tokens;
		} catch (err) {
			console.error(err);
		}

		return false;
	}

	let attachments = [];

	function buildFileElement(file, callback) {
		// file wrapper
		const _file = make("div", "file");

		// file name
		const _name = make("div", "name");

		_name.title = `FILE ${JSON.stringify(file.name)} LINES ${lines(file.content)}`;
		_name.textContent = file.name;

		_file.appendChild(_name);

		// token count
		if ("tokens" in file && Number.isInteger(file.tokens)) {
			const _tokens = make("div", "tokens");

			_tokens.textContent = `~${new Intl.NumberFormat("en-US").format(file.tokens)} tokens`;

			_file.appendChild(_tokens);
		}

		// remove button
		const _remove = make("button", "remove");

		_remove.title = "Remove attachment";

		_file.appendChild(_remove);

		_remove.addEventListener("click", () => {
			callback(_file);
		});

		return _file;
	}

	function pushAttachment(file) {
		file.id = uid();

		if (activeMessage?.isUser()) {
			activeMessage.addFile(file);

			return;
		}

		attachments.push(file);

		storeValue("attachments", attachments);

		$attachments.appendChild(
			buildFileElement(file, el => {
				const index = attachments.findIndex(attachment => attachment.id === file.id);

				if (index === -1) {
					return;
				}

				attachments.splice(index, 1);

				storeValue("attachments", attachments);

				el.remove();

				$attachments.classList.toggle("has-files", !!attachments.length);
			})
		);

		$attachments.classList.add("has-files");
	}

	function clearAttachments() {
		attachments = [];

		$attachments.innerHTML = "";
		$attachments.classList.remove("has-files");

		storeValue("attachments", []);
	}

	function pushMessage() {
		const text = $message.value.trim();

		if (!text && !attachments.length) {
			return false;
		}

		$message.value = "";
		storeValue("message", "");

		const message = new Message($role.value, "", text, false, attachments);

		clearAttachments();
		updateTitle();

		return message;
	}

	$total.addEventListener("auxclick", event => {
		if (event.button !== 1) {
			return;
		}

		totalCost = 0;

		updateTotalCost();
	});

	$titleRefresh.addEventListener("click", () => {
		refreshTitle();
	});

	$messages.addEventListener("scroll", () => {
		updateScrollButton();
	});

	$messages.addEventListener("wheel", event => {
		if (event.deltaY < 0) {
			setFollowTail(false);
		} else {
			setFollowTail(distanceFromBottom() - event.deltaY <= nearBottom);
		}
	});

	$bottom.addEventListener("click", () => {
		setFollowTail(true);

		$messages.scroll({
			top: $messages.scrollHeight,
			behavior: "smooth",
		});
	});

	$top.addEventListener("click", () => {
		setFollowTail($messages.scrollHeight <= $messages.clientHeight);

		$messages.scroll({
			top: 0,
			behavior: "smooth",
		});
	});

	$resizeBar.addEventListener("mousedown", event => {
		const isAtBottom = $messages.scrollHeight - ($messages.scrollTop + $messages.clientHeight) <= 10;

		if (event.button === 1) {
			$chat.style.height = "";

			storeValue("resized", false);

			scroll(isAtBottom, true);

			return;
		} else if (event.button !== 0) {
			return;
		}

		isResizing = true;
		scrollResize = isAtBottom;

		document.body.classList.add("resizing");
	});

	$role.addEventListener("change", () => {
		storeValue("role", $role.value);
	});

	$model.addEventListener("change", () => {
		const model = $model.value,
			data = model ? models[model] : null,
			tags = data?.tags || [];

		storeValue("model", model);

		if (tags.includes("reasoning")) {
			$reasoningEffort.parentNode.classList.remove("none");
			$reasoningTokens.parentNode.classList.toggle("none", !!$reasoningEffort.value);
		} else {
			$reasoningEffort.parentNode.classList.add("none");
			$reasoningTokens.parentNode.classList.add("none");
		}

		const hasJson = tags.includes("json"),
			hasSearch = searchAvailable && tags.includes("tools");

		$json.classList.toggle("none", !hasJson);
		$search.classList.toggle("none", !hasSearch);

		$search.parentNode.classList.toggle("none", !hasJson && !hasSearch);
	});

	$prompt.addEventListener("change", () => {
		storeValue("prompt", $prompt.value);
	});

	$temperature.addEventListener("input", () => {
		const value = $temperature.value,
			temperature = parseFloat(value);

		storeValue("temperature", value);

		$temperature.classList.toggle("invalid", Number.isNaN(temperature) || temperature < 0 || temperature > 2);
	});

	$iterations.addEventListener("input", () => {
		const value = $iterations.value,
			iterations = parseFloat(value);

		storeValue("iterations", value);

		$iterations.classList.toggle("invalid", Number.isNaN(iterations) || iterations < 1 || iterations > 50);
	});

	$reasoningEffort.addEventListener("change", () => {
		const effort = $reasoningEffort.value;

		storeValue("reasoning-effort", effort);

		$reasoningTokens.parentNode.classList.toggle("none", !!effort);
	});

	$reasoningTokens.addEventListener("input", () => {
		const value = $reasoningTokens.value,
			tokens = parseInt(value);

		storeValue("reasoning-tokens", value);

		$reasoningTokens.classList.toggle("invalid", Number.isNaN(tokens) || tokens <= 0 || tokens > 1024 * 1024);
	});

	$json.addEventListener("click", () => {
		jsonMode = !jsonMode;

		storeValue("json", jsonMode);

		$json.classList.toggle("on", jsonMode);
	});

	$search.addEventListener("click", () => {
		searchTool = !searchTool;

		storeValue("search", searchTool);

		$search.classList.toggle("on", searchTool);
	});

	$message.addEventListener("input", () => {
		storeValue("message", $message.value);
	});

	$upload.addEventListener("click", async () => {
		if (isUploading) {
			return;
		}

		const files = await selectFile(
			// the ultimate list
			"text/*",
			true,
			file => {
				if (!file.name) {
					file.name = "unknown.txt";
				} else if (file.name.length > 512) {
					throw new Error("File name too long (max 512 characters)");
				}

				if (!file.content) {
					throw new Error("File is empty");
				} else if (file.content.includes("\0")) {
					throw new Error("File is not a text file");
				} else if (file.content.length > 4 * 1024 * 1024) {
					throw new Error("File is too big (max 4MB)");
				}
			},
			notify
		);

		if (!files.length) {
			return;
		}

		isUploading = true;

		$upload.classList.add("loading");

		const promises = [];

		for (const file of files) {
			promises.push(
				resolveTokenCount(file.content).then(tokens => {
					file.tokens = tokens;
				})
			);
		}

		await Promise.all(promises);

		for (const file of files) {
			pushAttachment(file);
		}

		$upload.classList.remove("loading");

		isUploading = false;
	});

	$add.addEventListener("click", () => {
		pushMessage();
	});

	$clear.addEventListener("click", () => {
		if (!confirm("Are you sure you want to delete all messages?")) {
			return;
		}

		clearMessages();

		chatTitle = false;

		updateTitle();
	});

	$export.addEventListener("click", () => {
		const data = JSON.stringify({
			title: chatTitle,
			message: $message.value,
			attachments: attachments,
			role: $role.value,
			model: $model.value,
			prompt: $prompt.value,
			temperature: $temperature.value,
			iterations: $iterations.value,
			reasoning: {
				effort: $reasoningEffort.value,
				tokens: $reasoningTokens.value,
			},
			json: jsonMode,
			search: searchTool,
			messages: messages.map(message => message.getData(true)).filter(Boolean),
		});

		download("chat.json", "application/json", data);
	});

	$import.addEventListener("click", async () => {
		if (!modelList.length) {
			return;
		}

		const file = await selectFile(
				"application/json",
				false,
				file => {
					file.content = JSON.parse(file.content);
				},
				notify
			),
			data = file?.content;

		if (!data) {
			return;
		}

		clearMessages();

		storeValue("title", data.title);
		storeValue("message", data.message);
		storeValue("attachments", data.attachments);
		storeValue("role", data.role);
		storeValue("model", data.model);
		storeValue("prompt", data.prompt);
		storeValue("temperature", data.temperature);
		storeValue("iterations", data.iterations);
		storeValue("reasoning-effort", data.reasoning?.effort);
		storeValue("reasoning-tokens", data.reasoning?.tokens);
		storeValue("json", data.json);
		storeValue("search", data.search);
		storeValue("messages", data.messages);

		restore();
	});

	$scrolling.addEventListener("click", () => {
		autoScrolling = !autoScrolling;

		if (autoScrolling) {
			setFollowTail(true);

			$scrolling.title = "Turn off auto-scrolling";
			$scrolling.classList.add("on");

			scroll();
		} else {
			$scrolling.title = "Turn on auto-scrolling";
			$scrolling.classList.remove("on");
		}

		storeValue("scrolling", autoScrolling);
	});

	$send.addEventListener("click", () => {
		generate(true);
	});

	$login.addEventListener("click", async () => {
		$authentication.classList.remove("errored");
		$authentication.classList.add("loading");

		try {
			await login();

			$authentication.classList.remove("open");
		} catch (err) {
			$authError.textContent = `Error: ${err.message}`;
			$authentication.classList.add("errored");

			$password.value = "";
		}

		$authentication.classList.remove("loading");
	});

	$username.addEventListener("input", () => {
		$authentication.classList.remove("errored");
	});

	$password.addEventListener("input", () => {
		$authentication.classList.remove("errored");
	});

	$message.addEventListener("keydown", event => {
		if (event.shiftKey) {
			return;
		}

		if (event.ctrlKey && event.key === "Enter") {
			$send.click();
		}
	});

	addEventListener("mousemove", event => {
		if (!isResizing) {
			return;
		}

		const total = window.innerHeight,
			height = clamp(window.innerHeight - event.clientY + (attachments.length ? 50 : 0), 100, total - 240);

		$chat.style.height = `${height}px`;

		storeValue("resized", height);

		scroll(scrollResize, true);
	});

	addEventListener("mouseup", () => {
		isResizing = false;

		document.body.classList.remove("resizing");
	});

	addEventListener("keydown", event => {
		if (["TEXTAREA", "INPUT", "SELECT"].includes(document.activeElement?.tagName)) {
			return;
		}

		let delta;

		switch (event.key) {
			case "PageUp":
			case "ArrowUp":
				delta = event.key === "PageUp" ? -$messages.clientHeight : -120;

				setFollowTail(false);

				break;
			case "PageDown":
			case "ArrowDown":
				delta = event.key === "PageDown" ? $messages.clientHeight : 120;

				setFollowTail(distanceFromBottom() - delta <= nearBottom);

				break;
			case "Home":
				delta = -$messages.scrollTop;

				setFollowTail(false);

				break;
			case "End":
				delta = $messages.scrollHeight - $messages.clientHeight - $messages.scrollTop;

				setFollowTail(true);

				break;
		}

		if (delta) {
			event.preventDefault();

			$messages.scrollBy({
				top: delta,
				behavior: "smooth",
			});
		}
	});

	dropdown($role);
	dropdown($reasoningEffort);

	loadData().then(() => {
		restore();

		document.body.classList.remove("loading");
	});
})();
