(() => {
	const $messages = document.getElementById("messages"),
		$chat = document.getElementById("chat"),
		$message = document.getElementById("message"),
		$bottom = document.getElementById("bottom"),
		$role = document.getElementById("role"),
		$model = document.getElementById("model"),
		$temperature = document.getElementById("temperature"),
		$add = document.getElementById("add"),
		$send = document.getElementById("send"),
		$clear = document.getElementById("clear");

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

			return;
		}

		models.sort((a, b) => a.name > b.name);

		$model.innerHTML = "";

		for (const model of models) {
			const el = document.createElement("option");

			el.value = model.id;
			el.textContent = model.name;

			$model.appendChild(el);
		}
	}

	function restore(models) {
		$role.value = localStorage.getItem("role") || "user";
		$model.value = localStorage.getItem("model") || models[0].id;
		$temperature.value = localStorage.getItem("temperature") || 0.85;

		try {
			const messages = JSON.parse(localStorage.getItem("messages") || "[]");

			messages.forEach(addMessage);
		} catch {}
	}

	function saveMessages() {
		localStorage.setItem("messages", JSON.stringify(buildMessages(false)));
	}

	function scrollMessages() {
		$messages.scroll({
			top: $messages.scrollHeight + 200,
			behavior: "smooth",
		});
	}

	function toggleEditing(el) {
		const text = el.querySelector("div.text"),
			edit = el.querySelector("textarea.text");

		if (el.classList.contains("editing")) {
			text.textContent = edit.value.trim();

			el.classList.remove("editing");

			saveMessages();
		} else {
			edit.value = text.textContent;
			edit.style.height = `${text.offsetHeight}px`;

			el.classList.add("editing");

			edit.focus();
		}
	}

	function addMessage(message) {
		const el = document.createElement("div");

		el.classList.add("message", message.role);

		// message role
		const role = document.createElement("div");

		role.textContent = message.role;
		role.classList.add("role");

		el.appendChild(role);

		// message content
		const text = document.createElement("div");

		text.textContent = message.content;
		text.classList.add("text");

		el.appendChild(text);

		// message edit textarea
		const edit = document.createElement("textarea");

		edit.classList.add("text");

		el.appendChild(edit);

		// message options
		const opts = document.createElement("div");

		opts.classList.add("options");

		el.appendChild(opts);

		// edit option
		const optEdit = document.createElement("button");

		optEdit.title = "Edit message content";
		optEdit.classList.add("edit");

		opts.appendChild(optEdit);

		optEdit.addEventListener("click", () => {
			toggleEditing(el);
		});

		// delete option
		const optDelete = document.createElement("button");

		optDelete.title = "Delete message";
		optDelete.classList.add("delete");

		opts.appendChild(optDelete);

		optDelete.addEventListener("click", () => {
			el.remove();

			saveMessages();
		});

		// append to messages
		$messages.appendChild(el);

		scrollMessages();

		return {
			set(content) {
				text.textContent = content;

				scrollMessages();
			},
			state(state) {
				if (state && el.classList.contains(state)) {
					return;
				}

				el.classList.remove("waiting", "reasoning", "receiving");

				if (state) {
					el.classList.add(state);
				}

				scrollMessages();
			},
		};
	}

	function pushMessage() {
		const text = $message.value.trim();

		if (!text) {
			return false;
		}

		addMessage({
			role: $role.value,
			content: text,
		});

		$message.value = "";

		saveMessages();

		return true;
	}

	function buildMessages(clean = true) {
		const messages = [];

		$messages.querySelectorAll(".message").forEach((message) => {
			if (clean && message.classList.contains("editing")) {
				toggleEditing(message);
			}

			const role = message.querySelector(".role"),
				text = message.querySelector(".text");

			if (!role || !text) {
				return;
			}

			messages.push({
				role: role.textContent.trim(),
				content: text.textContent.trim().replace(/\r/g, ""),
			});
		});

		return messages;
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
		$messages.scroll({
			top: $messages.scrollHeight,
			behavior: "smooth",
		});
	});

	$role.addEventListener("change", () => {
		localStorage.setItem("role", $role.value);
	});

	$model.addEventListener("change", () => {
		localStorage.setItem("model", $model.value);
	});

	$temperature.addEventListener("input", () => {
		localStorage.setItem("temperature", $temperature.value);
	});

	$message.addEventListener("input", () => {
		localStorage.setItem("message", $message.value);
	});

	$add.addEventListener("click", () => {
		pushMessage();
	});

	$clear.addEventListener("click", () => {
		if (!confirm("Are you sure you want to delete all messages?")) {
			return;
		}

		$messages.innerHTML = "";

		saveMessages();
	});

	$send.addEventListener("click", () => {
		if (controller) {
			controller.abort();

			return;
		}

		const temperature = parseFloat($temperature.value);

		if (Number.isNaN(temperature) || temperature < 0 || temperature > 1) {
			return;
		}

		pushMessage();
		saveMessages();

		controller = new AbortController();

		$chat.classList.add("completing");

		const body = {
			model: $model.value,
			temperature: temperature,
			messages: buildMessages(),
		};

		const result = {
			role: "assistant",
			content: "",
		};

		const message = addMessage(result);

		message.state("waiting");

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

					saveMessages();

					$chat.classList.remove("completing");

					return;
				}

				switch (chunk.type) {
					case "reason":
						message.state("reasoning");

						break;
					case "text":
						result.content += chunk.text;

						message.state("receive");
						message.set(result.content);

						break;
				}

				saveMessages();
			},
		);
	});

	$message.addEventListener("keydown", (event) => {
		if (!event.ctrlKey || event.key !== "Enter") {
			return;
		}

		$send.click();
	});

	loadModels().then(restore);
})();
