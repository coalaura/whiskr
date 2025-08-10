(() => {
	const $messages = document.getElementById("messages"),
		$chat = document.getElementById("chat"),
		$message = document.getElementById("message"),
		$bottom = document.getElementById("bottom"),
		$role = document.getElementById("role"),
		$model = document.getElementById("model"),
		$prompt = document.getElementById("prompt"),
		$temperature = document.getElementById("temperature"),
		$add = document.getElementById("add"),
		$send = document.getElementById("send"),
		$scrolling = document.getElementById("scrolling"),
		$clear = document.getElementById("clear");

	const messages = [];

	let autoScrolling = false,
		interacted = false;

	function scroll(force = false) {
		if (!autoScrolling && !force) {
			return;
		}

		setTimeout(() => {
			$messages.scroll({
				top: $messages.scrollHeight + 200,
				behavior: "smooth",
			});
		}, 0);
	}

	class Message {
		#id;
		#role;
		#reasoning;
		#text;

		#editing = false;
		#expanded = false;
		#state = false;

		#_diff;
		#pending = {};
		#patching = {};

		#_message;
		#_role;
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

			// message role
			this.#_role = make("div", "role", this.#role);

			this.#_message.appendChild(this.#_role);

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
			if (!only || only === "role") {
				this.#_role.textContent = this.#role;
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
				this.#patch("text", this.#_text, this.#text, () => {
					noScroll || scroll();
				});

				this.#_message.classList.toggle("has-text", !!this.#text);
			}
		}

		#save() {
			storeValue(
				"messages",
				messages.map((message) => message.getData(true)),
			);
		}

		getData(includeReasoning = false) {
			const data = {
				role: this.#role,
				text: this.#text,
			};

			if (this.#reasoning && includeReasoning) {
				data.reasoning = this.#reasoning;
			}

			return data;
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
				throw new Error(response.statusText);
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
		const models = await json("/-/models");

		if (!models) {
			alert("Failed to load models.");

			return [];
		}

		$model.innerHTML = "";

		for (const model of models) {
			const el = document.createElement("option");

			el.value = model.id;
			el.title = model.description;
			el.textContent = model.name;

			el.dataset.tags = (model.tags || []).join(",");

			$model.appendChild(el);
		}

		dropdown($model);

		return models;
	}

	function restore(models) {
		$role.value = loadValue("role", "user");
		$model.value = loadValue("model", models[0].id);
		$prompt.value = loadValue("prompt", "normal");
		$temperature.value = loadValue("temperature", 0.85);

		if (loadValue("scrolling")) {
			$scrolling.click();
		}

		loadValue("messages", []).forEach(
			(message) => new Message(message.role, message.reasoning, message.text),
		);

		scroll(true);
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
		storeValue("model", $model.value);
	});

	$prompt.addEventListener("change", () => {
		storeValue("prompt", $prompt.value);
	});

	$temperature.addEventListener("input", () => {
		storeValue("temperature", $temperature.value);
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

		const temperature = parseFloat($temperature.value);

		if (Number.isNaN(temperature) || temperature < 0 || temperature > 1) {
			return;
		}

		pushMessage();

		controller = new AbortController();

		$chat.classList.add("completing");

		const body = {
			prompt: $prompt.value,
			model: $model.value,
			temperature: temperature,
			messages: messages.map((message) => message.getData()),
		};

		const message = new Message("assistant", "", "");

		message.setState("waiting");

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

	loadModels().then(restore);
})();
