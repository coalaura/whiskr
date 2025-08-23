(() => {
	const $version = document.getElementById("version"),
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

	const messages = [],
		models = {},
		modelList = [],
		promptList = [];

	let autoScrolling = false,
		jsonMode = false,
		searchTool = false;

	let searchAvailable = false,
		activeMessage = null,
		isResizing = false,
		scrollResize = false;

	function updateScrollButton() {
		const bottom = $messages.scrollHeight - ($messages.scrollTop + $messages.offsetHeight);

		$top.classList.toggle("hidden", $messages.scrollTop < 80);
		$bottom.classList.toggle("hidden", bottom < 80);
	}

	function scroll(force = false, instant = false) {
		if (!autoScrolling && !force) {
			updateScrollButton();

			return;
		}

		setTimeout(() => {
			$messages.scroll({
				top: $messages.scrollHeight,
				behavior: instant ? "instant" : "smooth",
			});
		}, 0);
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
		#id;
		#role;
		#reasoning;
		#text;
		#files = [];

		#tool;
		#tags = [];
		#statistics;
		#error = false;

		#editing = false;
		#expanded = false;
		#state = false;

		#_diff;
		#pending = {};
		#patching = {};

		#_message;
		#_tags;
		#_files;
		#_reasoning;
		#_text;
		#_edit;
		#_tool;
		#_statistics;

		constructor(role, reasoning, text, files = []) {
			this.#id = uid();
			this.#role = role;
			this.#reasoning = reasoning || "";
			this.#text = text || "";

			this.#_diff = document.createElement("div");

			this.#build();
			this.#render();

			for (const file of files) {
				this.addFile(file);
			}

			messages.push(this);

			if (this.#reasoning || this.#text) {
				this.#save();
			}
		}

		#build() {
			// main message div
			this.#_message = make("div", "message", this.#role);

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
				this.#expanded = !this.#expanded;

				this.#_message.classList.toggle("expanded", this.#expanded);

				if (this.#expanded) {
					this.#updateReasoningHeight();
				}

				updateScrollButton();
			});

			// message reasoning (height wrapper)
			const _height = make("div", "reasoning-wrapper");

			_reasoning.appendChild(_height);

			// message reasoning (content)
			this.#_reasoning = make("div", "reasoning-text", "markdown");

			_height.appendChild(this.#_reasoning);

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

			// message tool
			this.#_tool = make("div", "tool");

			_body.appendChild(this.#_tool);

			// tool call
			const _call = make("div", "call");

			this.#_tool.appendChild(_call);

			_call.addEventListener("click", () => {
				this.#_tool.classList.toggle("expanded");

				updateScrollButton();
			});

			// tool call name
			const _callName = make("div", "name");

			_call.appendChild(_callName);

			// tool call arguments
			const _callArguments = make("div", "arguments");

			_call.appendChild(_callArguments);

			// tool call result
			const _callResult = make("div", "result", "markdown");

			this.#_tool.appendChild(_callResult);

			// message options
			const _opts = make("div", "options");

			this.#_message.appendChild(_opts);

			// copy option
			const _optCopy = make("button", "copy");

			_optCopy.title = "Copy message content";

			_opts.appendChild(_optCopy);

			let timeout;

			_optCopy.addEventListener("click", () => {
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

				while (messages.length > index) {
					messages[messages.length - 1].delete();
				}

				mark(false);

				generate(false);
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
			this.#_reasoning.parentNode.style.setProperty("--height", `${this.#_reasoning.scrollHeight}px`);
		}

		#updateToolHeight() {
			const result = this.#_tool.querySelector(".result");

			this.#_tool.style.setProperty("--height", `${result.scrollHeight}px`);
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

			if (!only || only === "tool") {
				if (this.#tool) {
					const { name, args, result } = this.#tool;

					const _name = this.#_tool.querySelector(".name"),
						_arguments = this.#_tool.querySelector(".arguments"),
						_result = this.#_tool.querySelector(".result");

					_name.title = `Show ${name} call result`;
					_name.textContent = name;

					_arguments.title = args;
					_arguments.textContent = args;

					_result.innerHTML = render(result || "*processing*");

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

					const tps = output / (time / 1000),
						price = cost < 1 ? `${fixed(cost * 100, 1)}ct` : `$${fixed(cost, 2)}`;

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
						`<div class="cost">${price}</div>`,
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
				}));
			}

			if (this.#tool) {
				data.tool = this.#tool;
			}

			if (this.#reasoning && full) {
				data.reasoning = this.#reasoning;
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

			if (!data.files?.length && !data.reasoning && !data.text && !data.tool) {
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
			if (!generationID) {
				return;
			}

			try {
				const response = await fetch(`/-/stats/${generationID}`),
					data = await response.json();

				if (!data || data.error) {
					throw new Error(data?.error || response.statusText);
				}

				this.setStatistics(data);
			} catch (err) {
				console.error(err);

				if (!retrying && err.message.includes("not found")) {
					setTimeout(this.loadGenerationData.bind(this), 750, generationID, true);
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

		showError(error) {
			this.#error = error;

			this.#_message.classList.add("errored");

			const _err = make("div", "error");

			_err.textContent = this.#error;

			this.#_text.appendChild(_err);

			this.#save();
		}

		stopEdit() {
			if (!this.#editing) {
				return;
			}

			this.toggleEdit();
		}

		toggleEdit() {
			this.#editing = !this.#editing;

			if (this.#editing) {
				activeMessage = this;

				this.#_edit.value = this.#text;

				this.#_edit.style.height = `${this.#_text.offsetHeight}px`;
				this.#_edit.style.width = `${this.#_text.offsetWidth}px`;

				this.setState("editing");

				this.#_edit.focus();
			} else {
				activeMessage = null;

				this.#text = this.#_edit.value;

				this.setState(false);

				this.#render(false, true);
				this.#save();
			}
		}

		delete() {
			const index = messages.findIndex(msg => msg.#id === this.#id);

			if (index === -1) {
				return;
			}

			this.#_message.remove();

			messages.splice(index, 1);

			this.#save();

			$messages.dispatchEvent(new Event("scroll"));
		}
	}

	let controller;

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
		try {
			const response = await fetch(url, options);

			if (!response.ok) {
				const err = await response.json();

				if (err?.error === "unauthorized") {
					showLogin();
				}

				throw new Error(err?.error || response.statusText);
			}

			const reader = response.body.getReader(),
				decoder = new TextDecoder();

			let buffer = "";

			while (true) {
				const { value, done } = await reader.read();

				if (done) break;

				buffer += decoder.decode(value, {
					stream: true,
				});

				while (true) {
					const idx = buffer.indexOf("\n\n");

					if (idx === -1) {
						break;
					}

					const frame = buffer.slice(0, idx).trim();
					buffer = buffer.slice(idx + 2);

					if (!frame) {
						continue;
					}

					try {
						const chunk = JSON.parse(frame);

						if (!chunk) {
							throw new Error("invalid chunk");
						}

						callback(chunk);
					} catch (err) {
						console.warn("bad frame", frame);
						console.warn(err);
					}
				}
			}
		} catch (err) {
			if (err.name !== "AbortError") {
				callback({
					type: "error",
					text: err.message,
				});
			}
		} finally {
			callback(false);
		}
	}

	function generate(cancel = false) {
		if (controller) {
			controller.abort();

			if (cancel) {
				return;
			}
		}

		if (!$temperature.value) {
		}

		let temperature = parseFloat($temperature.value);

		if (Number.isNaN(temperature) || temperature < 0 || temperature > 2) {
			temperature = 0.85;

			$temperature.value = temperature;
			$temperature.classList.remove("invalid");
		}

		let iterations = parseInt($iterations.value);

		if (Number.isNaN(iterations) || iterations < 1 || iterations > 50) {
			iterations = 3;

			$iterations.value = iterations;
			$iterations.classList.remove("invalid");
		}

		const effort = $reasoningEffort.value;

		let tokens = parseInt($reasoningTokens.value);

		if (!effort && (Number.isNaN(tokens) || tokens <= 0 || tokens > 1024 * 1024)) {
			tokens = 1024;

			$reasoningTokens.value = tokens;
			$reasoningTokens.classList.remove("invalid");
		}

		pushMessage();

		controller = new AbortController();

		$chat.classList.add("completing");

		const body = {
			prompt: $prompt.value,
			model: $model.value,
			temperature: temperature,
			iterations: iterations,
			reasoning: {
				effort: effort,
				tokens: tokens || 0,
			},
			json: jsonMode,
			search: searchTool,
			messages: messages.map(message => message.getData()).filter(Boolean),
		};

		let message, generationID;

		function finish() {
			if (!message) {
				return;
			}

			message.setState(false);

			setTimeout(message.loadGenerationData.bind(message), 750, generationID);

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
				signal: controller.signal,
			},
			chunk => {
				if (!chunk) {
					controller = null;

					finish();

					$chat.classList.remove("completing");

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
						generationID = chunk.text;

						break;
					case "tool":
						message.setState("tooling");
						message.setTool(chunk.text);

						if (chunk.text.done) {
							finish();
						}

						break;
					case "reason":
						message.setState("reasoning");
						message.addReasoning(chunk.text);

						break;
					case "text":
						message.setState("receiving");
						message.addText(chunk.text);

						break;
					case "error":
						message.showError(chunk.text);

						break;
				}
			}
		);
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

	async function loadData() {
		const data = await json("/-/data");

		if (!data) {
			alert("Failed to load data.");

			return false;
		}

		// start icon preload
		preloadIcons(data.icons);

		// render version
		if (data.version === "dev") {
			$version.remove();
		} else {
			$version.innerHTML = `<a href="https://github.com/coalaura/whiskr" target="_blank">whiskr</a> <a href="https://github.com/coalaura/whiskr/releases/tag/${data.version}" target="_blank">${data.version}</a>`;
		}

		// update search availability
		searchAvailable = data.search;

		// show login modal
		if (data.authentication && !data.authenticated) {
			$authentication.classList.add("open");
		}

		// render models
		fillSelect($model, data.models, (el, model) => {
			el.value = model.id;
			el.title = model.description;
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

		return data;
	}

	function clearMessages() {
		while (messages.length) {
			messages[0].delete();
		}
	}

	function restore() {
		$message.value = loadValue("message", "");
		$role.value = loadValue("role", "user");
		$model.value = loadValue("model", modelList[0].id);
		$prompt.value = loadValue("prompt", promptList[0].key);
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
			const obj = new Message(message.role, message.reasoning, message.text, message.files || []);

			if (message.error) {
				obj.showError(message.error);
			}

			if (message.tags) {
				message.tags.forEach(tag => obj.addTag(tag));
			}

			if (message.tool) {
				obj.setTool(message.tool);
			}

			if (message.statistics) {
				obj.setStatistics(message.statistics);
			}
		});

		scroll();

		// small fix, sometimes when hard reloading we don't scroll all the way
		setTimeout(scroll, 250);
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

		const message = new Message($role.value, "", text, attachments);

		clearAttachments();

		return message;
	}

	$messages.addEventListener("scroll", () => {
		updateScrollButton();
	});

	$bottom.addEventListener("click", () => {
		$messages.scroll({
			top: $messages.scrollHeight,
			behavior: "smooth",
		});
	});

	$top.addEventListener("click", () => {
		$messages.scroll({
			top: 0,
			behavior: "smooth",
		});
	});

	$resizeBar.addEventListener("mousedown", event => {
		const isAtBottom = $messages.scrollHeight - ($messages.scrollTop + $messages.offsetHeight) <= 10;

		if (event.buttons === 4) {
			$chat.style.height = "";

			storeValue("resized", false);

			scroll(isAtBottom, true);

			return;
		} else if (event.buttons !== 1) {
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
		const file = await selectFile(
			// the ultimate list
			".adoc,.bash,.bashrc,.bat,.c,.cc,.cfg,.cjs,.cmd,.conf,.cpp,.cs,.css,.csv,.cxx,.dockerfile,.dockerignore,.editorconfig,.env,.fish,.fs,.fsx,.gitattributes,.gitignore,.go,.gradle,.groovy,.h,.hh,.hpp,.htm,.html,.ini,.ipynb,.java,.jl,.js,.json,.jsonc,.jsx,.kt,.kts,.less,.log,.lua,.m,.makefile,.markdown,.md,.mjs,.mk,.mm,.php,.phtml,.pl,.pm,.profile,.properties,.ps1,.psql,.py,.pyw,.r,.rb,.rs,.rst,.sass,.scala,.scss,.sh,.sql,.svelte,.swift,.t,.toml,.ts,.tsv,.tsx,.txt,.vb,.vue,.xhtml,.xml,.xsd,.xsl,.xslt,.yaml,.yml,.zig,.zsh",
			false
		);

		if (!file) {
			return;
		}

		try {
			if (!file.name) {
				file.name = "unknown.txt";
			} else if (file.name.length > 512) {
				throw new Error("File name too long (max 512 characters)");
			}

			if (typeof file.content !== "string") {
				throw new Error("File is not a text file");
			} else if (!file.content) {
				throw new Error("File is empty");
			} else if (file.content.length > 4 * 1024 * 1024) {
				throw new Error("File is too big (max 4MB)");
			}

			pushAttachment(file);
		} catch (err) {
			alert(err.message);
		}
	});

	$add.addEventListener("click", () => {
		pushMessage();
	});

	$clear.addEventListener("click", () => {
		if (!confirm("Are you sure you want to delete all messages?")) {
			return;
		}

		clearMessages();
	});

	$export.addEventListener("click", () => {
		const data = JSON.stringify({
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
			messages: messages.map(message => message.getData()).filter(Boolean),
		});

		download("chat.json", "application/json", data);
	});

	$import.addEventListener("click", async () => {
		if (!modelList.length) {
			return;
		}

		const file = await selectFile("application/json", true),
			data = file?.content;

		if (!data) {
			return;
		}

		clearMessages();

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
			height = clamp(window.innerHeight - event.clientY, 100, total - 240);

		$chat.style.height = `${height}px`;

		storeValue("resized", height);

		scroll(scrollResize, true);
	});

	addEventListener("mouseup", () => {
		isResizing = false;

		document.body.classList.remove("resizing");
	});

	dropdown($role);
	dropdown($reasoningEffort);

	const resizedHeight = loadValue("resized");

	if (resizedHeight) {
		$chat.style.height = `${resizedHeight}px`;
	}

	loadData().then(() => {
		restore();

		document.body.classList.remove("loading");

		setTimeout(() => {
			document.getElementById("loading").remove();
		}, 500);
	});
})();
