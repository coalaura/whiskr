(() => {
	const $messages = document.getElementById("messages"),
		$chat = document.getElementById("chat"),
		$message = document.getElementById("message"),
		$bottom = document.getElementById("bottom"),
		$role = document.getElementById("role"),
		$model = document.getElementById("model"),
		$prompt = document.getElementById("prompt"),
		$temperature = document.getElementById("temperature"),
		$reasoningEffort = document.getElementById("reasoning-effort"),
		$reasoningTokens = document.getElementById("reasoning-tokens"),
		$json = document.getElementById("json"),
		$search = document.getElementById("search"),
		$add = document.getElementById("add"),
		$send = document.getElementById("send"),
		$scrolling = document.getElementById("scrolling"),
		$clear = document.getElementById("clear");

	const messages = [],
		models = {};

	let autoScrolling = false,
		jsonMode = false,
		searchTool = false,
		interacted = false;

	function scroll(force = false) {
		if (!autoScrolling && !force) {
			return;
		}

		setTimeout(() => {
			$messages.scroll({
				top: $messages.scrollHeight,
				behavior: "smooth",
			});
		}, 0);
	}

	class Message {
		#id;
		#role;
		#reasoning;
		#text;

		#tags = [];
		#error = false;

		#editing = false;
		#expanded = false;
		#state = false;

		#_diff;
		#pending = {};
		#patching = {};

		#_message;
		#_tags;
		#_reasoning;
		#_text;
		#_edit;

		constructor(role, reasoning, text) {
			this.#id = uid();
			this.#role = role;
			this.#reasoning = reasoning || "";
			this.#text = text || "";

			this.#_diff = document.createElement("div");

			this.#build();
			this.#render();

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

			// message reasoning (wrapper)
			const _reasoning = make("div", "reasoning");

			this.#_message.appendChild(_reasoning);

			// message reasoning (toggle)
			const _toggle = make("button", "toggle");

			_toggle.textContent = "Reasoning";

			_reasoning.appendChild(_toggle);

			_toggle.addEventListener("click", () => {
				this.#expanded = !this.#expanded;

				if (this.#expanded) {
					this.#_message.classList.add("expanded");
				} else {
					this.#_message.classList.remove("expanded");
				}
			});

			// message reasoning (content)
			this.#_reasoning = make("div", "reasoning-text", "markdown");

			_reasoning.appendChild(this.#_reasoning);

			// message content
			this.#_text = make("div", "text", "markdown");

			this.#_message.appendChild(this.#_text);

			// message edit textarea
			this.#_edit = make("textarea", "text");

			this.#_message.appendChild(this.#_edit);

			this.#_edit.addEventListener("keydown", (event) => {
				if (event.ctrlKey && event.key === "Enter") {
					this.toggleEdit();
				} else if (event.key === "Escape") {
					this.#_edit.value = this.#text;

					this.toggleEdit();
				}
			});

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

			// add to dom
			$messages.appendChild(this.#_message);

			scroll();
		}

		#handleImages(element) {
			element.querySelectorAll("img:not(.image)").forEach((img) => {
				img.classList.add("image");

				img.addEventListener("load", () => {
					scroll(!interacted);
				});
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

				morphdom(element, this.#_diff, {
					childrenOnly: true,
					onBeforeElUpdated: (fromEl, toEl) => {
						return !fromEl.isEqualNode || !fromEl.isEqualNode(toEl);
					},
				});

				this.#_diff.innerHTML = "";

				this.#handleImages(element);

				after?.();
			});
		}

		#render(only = false, noScroll = false) {
			if (!only || only === "tags") {
				console.log(this.#tags);
				this.#_tags.innerHTML = this.#tags
					.map((tag) => `<div class="tag-${tag}" title="${tag}"></div>`)
					.join("");

				this.#_message.classList.toggle("has-tags", this.#tags.length > 0);
			}

			if (this.#error) {
				return;
			}

			if (!only || only === "reasoning") {
				this.#patch("reasoning", this.#_reasoning, this.#reasoning, () => {
					this.#_reasoning.style.setProperty(
						"--height",
						`${this.#_reasoning.scrollHeight}px`,
					);

					noScroll || scroll();
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
				});

				this.#_message.classList.toggle("has-text", !!this.#text);
			}
		}

		#save() {
			storeValue(
				"messages",
				messages.map((message) => message.getData(true)).filter(Boolean),
			);
		}

		getData(full = false) {
			const data = {
				role: this.#role,
				text: this.#text,
			};

			if (this.#reasoning && full) {
				data.reasoning = this.#reasoning;
			}

			if (this.#error && full) {
				data.error = this.#error;
			}

			if (this.#tags.length && full) {
				data.tags = this.#tags;
			}

			return data;
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

		setState(state) {
			if (this.#state === state) {
				return;
			}

			if (this.#state) {
				this.#_message.classList.remove(this.#state);
			}

			if (state) {
				this.#_message.classList.add(state);
			}

			this.#state = state;
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
				this.#_edit.value = this.#text;

				this.#_edit.style.height = `${this.#_text.offsetHeight}px`;
				this.#_edit.style.width = `${this.#_text.offsetWidth}px`;

				this.setState("editing");

				this.#_edit.focus();
			} else {
				this.#text = this.#_edit.value;

				this.setState(false);

				this.#render(false, true);
				this.#save();
			}
		}

		delete() {
			const index = messages.findIndex((msg) => msg.#id === this.#id);

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

	async function loadModels() {
		const modelList = await json("/-/models");

		if (!modelList) {
			alert("Failed to load models.");

			return [];
		}

		$model.innerHTML = "";

		for (const model of modelList) {
			const el = document.createElement("option");

			el.value = model.id;
			el.title = model.description;
			el.textContent = model.name;

			el.dataset.tags = (model.tags || []).join(",");

			$model.appendChild(el);

			models[model.id] = model;
		}

		dropdown($model, 4);

		return modelList;
	}

	function restore(modelList) {
		$role.value = loadValue("role", "user");
		$model.value = loadValue("model", modelList[0].id);
		$prompt.value = loadValue("prompt", "normal");
		$temperature.value = loadValue("temperature", 0.85);
		$reasoningEffort.value = loadValue("reasoning-effort", "medium");
		$reasoningTokens.value = loadValue("reasoning-tokens", 1024);

		if (loadValue("json")) {
			$json.click();
		}

		if (loadValue("search")) {
			$search.click();
		}

		if (loadValue("scrolling")) {
			$scrolling.click();
		}

		loadValue("messages", []).forEach((message) => {
			const obj = new Message(message.role, message.reasoning, message.text);

			if (message.error) {
				obj.showError(message.error);
			}

			if (message.tags) {
				message.tags.forEach((tag) => obj.addTag(tag));
			}
		});

		scroll(true);

		// small fix, sometimes when hard reloading we don't scroll all the way
		setTimeout(scroll, 250, true);
	}

	function pushMessage() {
		const text = $message.value.trim();

		if (!text) {
			return false;
		}

		$message.value = "";

		return new Message($role.value, "", text);
	}

	$messages.addEventListener("scroll", () => {
		const bottom =
			$messages.scrollHeight - ($messages.scrollTop + $messages.offsetHeight);

		if (bottom >= 80) {
			$bottom.classList.remove("hidden");
		} else {
			$bottom.classList.add("hidden");
		}
	});

	$bottom.addEventListener("click", () => {
		interacted = true;

		scroll(true);
	});

	$role.addEventListener("change", () => {
		storeValue("role", $role.value);
	});

	$model.addEventListener("change", () => {
		const model = $model.value,
			data = model ? models[model] : null;

		storeValue("model", model);

		if (data?.tags.includes("reasoning")) {
			$reasoningEffort.parentNode.classList.remove("none");
			$reasoningTokens.parentNode.classList.toggle(
				"none",
				!!$reasoningEffort.value,
			);
		} else {
			$reasoningEffort.parentNode.classList.add("none");
			$reasoningTokens.parentNode.classList.add("none");
		}

		$json.classList.toggle("none", !data?.tags.includes("json"));
	});

	$prompt.addEventListener("change", () => {
		storeValue("prompt", $prompt.value);
	});

	$temperature.addEventListener("input", () => {
		const value = $temperature.value,
			temperature = parseFloat(value);

		storeValue("temperature", value);

		$temperature.classList.toggle(
			"invalid",
			Number.isNaN(temperature) || temperature < 0 || temperature > 2,
		);
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

		$reasoningTokens.classList.toggle(
			"invalid",
			Number.isNaN(tokens) || tokens <= 0 || tokens > 1024 * 1024,
		);
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

	$add.addEventListener("click", () => {
		interacted = true;

		pushMessage();
	});

	$clear.addEventListener("click", () => {
		if (!confirm("Are you sure you want to delete all messages?")) {
			return;
		}

		interacted = true;

		for (let x = messages.length - 1; x >= 0; x--) {
			messages[x].delete();
		}
	});

	$scrolling.addEventListener("click", () => {
		interacted = true;

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
		interacted = true;

		if (controller) {
			controller.abort();

			return;
		}

		if (!$temperature.value) {
			$temperature.value = 0.85;
		}

		const temperature = parseFloat($temperature.value);

		if (Number.isNaN(temperature) || temperature < 0 || temperature > 2) {
			return;
		}

		const effort = $reasoningEffort.value,
			tokens = parseInt($reasoningTokens.value);

		if (
			!effort &&
			(Number.isNaN(tokens) || tokens <= 0 || tokens > 1024 * 1024)
		) {
			return;
		}

		pushMessage();

		controller = new AbortController();

		$chat.classList.add("completing");

		const body = {
			prompt: $prompt.value,
			model: $model.value,
			temperature: temperature,
			reasoning: {
				effort: effort,
				tokens: tokens || 0,
			},
			json: jsonMode,
			search: searchTool,
			messages: messages.map((message) => message.getData()),
		};

		const message = new Message("assistant", "", "");

		message.setState("waiting");

		if (jsonMode) {
			message.addTag("json");
		}

		if (searchTool) {
			message.addTag("search");
		}

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
			(chunk) => {
				if (!chunk) {
					controller = null;

					message.setState(false);

					$chat.classList.remove("completing");

					return;
				}

				switch (chunk.type) {
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
			},
		);
	});

	$message.addEventListener("keydown", (event) => {
		if (!event.ctrlKey || event.key !== "Enter") {
			return;
		}

		$send.click();
	});

	addEventListener("wheel", () => {
		interacted = true;
	});

	dropdown($role);
	dropdown($prompt);
	dropdown($reasoningEffort);

	loadModels().then(restore);
})();
