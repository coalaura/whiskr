import "../css/chat.css";

import morphdom from "morphdom";
import { unpack } from "msgpackr";
import { getDataUrlAspectRatio } from "./binary.js";
import { dropdown } from "./dropdown.js";
import { resetGenerationState, setGenerationState } from "./favicon.js";
import {
	bHeight,
	clamp,
	dataUrlFilename,
	detectPlatform,
	download,
	fillSelect,
	fixed,
	formatBytes,
	formatMilliseconds,
	formatMoney,
	formatTimestamp,
	lines,
	make,
	maxImageDimension,
	notify,
	previewFile,
	readFileAsDataUrl,
	resizeDataUrl,
	schedule,
	selectFile,
	uid,
	wrapJSON,
} from "./lib.js";
import { render, renderInline, stripMarkdown } from "./markdown.js";
import { connectDB, load, onChange, refresh, store } from "./storage.js";

const ChunkType = {
	0: "start",
	1: "id",
	2: "reason",
	3: "reason_type",
	4: "text",
	5: "image",
	6: "tool",
	7: "error",
	8: "end",
	9: "alive",
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
	$role = document.getElementById("role").querySelector("select"),
	$model = document.getElementById("model"),
	$providerSorting = document.getElementById("provider-sorting"),
	$imageResolution = document.getElementById("image-resolution"),
	$imageAspect = document.getElementById("image-aspect"),
	$reasoningEffort = document.getElementById("reasoning-effort"),
	$reasoningTokens = document.getElementById("reasoning-tokens"),
	$prompt = document.getElementById("prompt"),
	$temperature = document.getElementById("temperature"),
	$iterations = document.getElementById("iterations"),
	$json = document.getElementById("json"),
	$search = document.getElementById("search"),
	$add = document.getElementById("add"),
	$send = document.getElementById("send"),
	$scrolling = document.getElementById("scrolling"),
	$upload = document.getElementById("upload"),
	$export = document.getElementById("export-sidebar"),
	$import = document.getElementById("import-sidebar"),
	$dump = document.getElementById("dump"),
	$clear = document.getElementById("clear"),
	$sidebar = document.getElementById("sidebar"),
	$sidebarTrigger = document.getElementById("sidebar-trigger"),
	$sidebarClose = document.getElementById("sidebar-close"),
	$sEnabled = document.getElementById("s-enabled"),
	$sName = document.getElementById("s-name"),
	$sPrompt = document.getElementById("s-prompt"),
	$personalizationSection = document.getElementById("personalization-section"),
	$personalizationHeader = document.getElementById("personalization-header"),
	$personalizationCollapse = document.getElementById("personalization-collapse"),
	$personalizationBody = document.getElementById("personalization-body"),
	$saveCurrentChat = document.getElementById("save-current-chat"),
	$savedChatsList = document.getElementById("saved-chats-list"),
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

const settings = {
	enabled: true,
	name: "",
	prompt: "",
};

const messages = [],
	models = {},
	modelList = [],
	disabledModels = [],
	promptList = [],
	pendingImages = new Map();

let autoScrolling = false,
	followTail = true,
	awaitingScroll = false,
	jsonMode = false,
	searchTool = false,
	chatTitle = false;

let searchAvailable = false,
	isResizing = false,
	scrollResize = false,
	isUploading = false,
	isDumping = false,
	usageType = "monthly",
	totalUsage = {};

function updateTotalUsage() {
	$total.textContent = `${usageType[0].toUpperCase()} / ${formatMoney(totalUsage[usageType] || 0)}`;

	const titles = {
		total: "Whiskr: All-time usage",
		monthly: "Whiskr: Usage this month",
		weekly: "Whiskr: Usage this week",
		daily: "Whiskr: Usage today",
	};

	$total.title = titles[usageType] || "Usage";
}

function updateTitle() {
	const title = chatTitle || "New Chat";

	$title.classList.toggle("hidden", !messages.length);

	$titleText.textContent = title;

	document.title = `whiskr${chatTitle ? ` - ${chatTitle}` : ""}`;

	store("title", chatTitle);
}

function updatePersonalizationVisualState() {
	const isDisabled = !settings.enabled;

	$personalizationSection.classList.toggle("disabled", isDisabled);

	$sName.disabled = isDisabled;
	$sPrompt.disabled = isDisabled;
}

async function syncSavedChats() {
	await refresh(["saved-chats"]);

	renderSavedChats();
}

let syncReady = false;

function setupCrossTabSync() {
	if (syncReady) {
		return;
	}

	syncReady = true;

	onChange(change => {
		if (!change || change.isLocal || !change.key) {
			return;
		}

		if (change.key === "saved-chats") {
			schedule(syncSavedChats);
		}
	});
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

	if (!document.hasFocus()) {
		$messages.scrollTop = $messages.scrollHeight;

		return;
	}

	awaitingScroll = true;

	requestAnimationFrame(() => {
		if (!followTail && !force) {
			return;
		}

		$messages.scroll({
			top: $messages.scrollHeight,
			behavior: instant ? "instant" : "smooth",
		});

		awaitingScroll = false;
	});
}

function mark(index) {
	for (let x = 0; x < messages.length; x++) {
		messages[x].mark(Number.isInteger(index) && x >= index);
	}
}

async function insertImageIntoTextarea(dataUrl, textarea) {
	const filename = await dataUrlFilename(dataUrl),
		hash = filename.replace(/\.[^/.]+$/, "");

	if (textarea === $message) {
		pendingImages.set(hash, dataUrl);
	} else {
		const msg = messages.find(_msg => _msg.getEditTextarea() === textarea);

		if (msg) {
			msg.addInlineImage(hash, dataUrl);
		}
	}

	const placeholder = `![img](${filename})`,
		start = textarea.selectionStart,
		end = textarea.selectionEnd,
		text = textarea.value;

	textarea.value = text.slice(0, start) + placeholder + text.slice(end);

	textarea.selectionStart = textarea.selectionEnd = start + placeholder.length;

	if (textarea === $message) {
		store("message", textarea.value);
	}
}

class Message {
	#destroyed = false;

	#id;
	#role;
	#reasoning;
	#reasoningType;
	#text;
	#images = [];
	#files = [];
	#inlineImages = new Map();

	#tool;
	#tags = [];
	#time = 0;
	#ttft = 0;
	#statistics;
	#error = false;

	#editing = false;
	#state = false;
	#loading = false;
	#inline = {};

	#_diff;
	#pending = {};
	#patching = {};

	#_message;
	#_tags;
	#_time;
	#_files;
	#_reasoning;
	#_text;
	#_edit;
	#_images;
	#_tool;
	#_statistics;
	#_roleSelect;

	constructor(data) {
		this.#id = uid();
		this.#role = data.role;
		this.#reasoning = data.reasoning || "";
		this.#reasoningType = data.reasoningType || "";
		this.#text = data.text || "";

		this.#time = data.time;
		this.#ttft = data.ttft;

		if (data.inlineImages) {
			if (data.inlineImages instanceof Map) {
				this.#inlineImages = new Map(data.inlineImages);
			} else if (Array.isArray(data.inlineImages)) {
				this.#inlineImages = new Map(data.inlineImages);
			}
		}

		this.#_diff = document.createElement("div");

		this.#build(data.collapsed);
		this.#render();

		if (data.tool?.name) {
			this.setTool(data.tool);
		}

		if (data.files) {
			for (const file of data.files) {
				this.addFile(file);
			}
		}

		if (data.images) {
			for (const image of data.images) {
				this.addImage(image);
			}
		}

		if (data.tags) {
			for (const tag of data.tags) {
				this.addTag(tag);
			}
		}

		if (data.statistics) {
			this.setStatistics(data.statistics);
		}

		if (data.error) {
			this.setError(data.error);
		}

		messages.push(this);

		this.#save();
	}

	#build(collapsed) {
		// main message div
		this.#_message = make("div", "message", this.#role, collapsed ? "collapsed" : "");

		// header
		const _header = make("div", "header");

		this.#_message.appendChild(_header);

		_header.addEventListener("auxclick", event => {
			if (event.button !== 1) {
				return;
			}

			this.#_message.scrollIntoView({
				behavior: "smooth",
				block: "start",
			});
		});

		// message role (wrapper)
		const _wrapper = make("div", "role", this.#role);

		_header.appendChild(_wrapper);

		// message role selector
		this.#_roleSelect = make("select");

		fillSelect(
			this.#_roleSelect,
			[
				{ value: "user", label: "user", selected: this.#role === "user" },
				{ value: "assistant", label: "assistant", selected: this.#role === "assistant" },
				{ value: "system", label: "system", selected: this.#role === "system" },
			],
			(el, option) => {
				el.value = option.value;
				el.textContent = option.label;

				if (option.selected) {
					el.selected = true;
				}
			}
		);

		_wrapper.appendChild(this.#_roleSelect);

		dropdown(this.#_roleSelect);

		this.#_roleSelect.addEventListener("change", () => {
			const newRole = this.#_roleSelect.value;

			if (this.#role === newRole) {
				return;
			}

			this.setRole(newRole);
		});

		// message tags
		this.#_tags = make("div", "tags");

		_wrapper.appendChild(this.#_tags);

		// message options
		const _opts = make("div", "options");

		_header.appendChild(_opts);

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

		// attach option
		if (this.#role === "user") {
			const _attach = make("button", "attach");

			_attach.title = "Add files to this message";

			_opts.appendChild(_attach);

			_attach.addEventListener("click", () => {
				uploadToMessage(_attach, this);
			});
		}

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
			const index = this.index(_assistant ? 0 : 1);

			mark(index);
		});

		_optRetry.addEventListener("mouseleave", () => {
			mark(false);
		});

		_optRetry.addEventListener("click", () => {
			const index = this.index(_assistant ? 0 : 1);

			if (index === false) {
				return;
			}

			abortNow();

			this.stopEdit();

			for (let i = 0; i < index; i++) {
				messages[i].stopEdit();
			}

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
			if (this.#_message.classList.contains("collapsed")) {
				_optCollapse.click();
			}

			this.toggleEdit();
		});

		// delete option
		const _optDelete = make("button", "delete");

		_optDelete.title = "Delete message";

		_opts.appendChild(_optDelete);

		_optDelete.addEventListener("click", () => {
			this.delete();
		});

		// message body
		const _body = make("div", "body");

		this.#_message.appendChild(_body);

		// time
		this.#_time = make("div", "time");

		_body.appendChild(this.#_time);

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

		this.#_text.addEventListener("click", event => {
			if (event.ctrlKey && !event.target.closest(".no-click") && !this.#state) {
				event.preventDefault();

				if (!this.#editing) {
					if (this.#_message.classList.contains("collapsed")) {
						this.#_message.classList.remove("collapsed");

						updateScrollButton();

						setFollowTail(distanceFromBottom() <= nearBottom);
					}

					this.toggleEdit();
				}

				return;
			}

			this.#handlePreview(event);
		});

		this.#_text.addEventListener("auxclick", event => {
			if (event.button !== 1) {
				return;
			}

			this.#handlePreview(event);
		});

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

		// paste in edit textarea
		this.#_edit.addEventListener("paste", async event => {
			const items = event.clipboardData?.items;

			if (!items) {
				return;
			}

			for (const item of items) {
				if (item.type.startsWith("image/")) {
					event.preventDefault();

					const file = item.getAsFile(),
						dataUrl = await readFileAsDataUrl(file);

					await insertImageIntoTextarea(dataUrl, this.#_edit);
				}
			}
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

		// statistics
		this.#_statistics = make("div", "statistics");

		this.#_message.appendChild(this.#_statistics);

		// scroll into view
		const _scroll = make("button", "scroll-view");

		_scroll.title = "Scroll message into view";

		this.#_message.appendChild(_scroll);

		_scroll.addEventListener("click", () => {
			this.#_message.scrollIntoView({
				behavior: "smooth",
				block: "start",
			});
		});

		// add to dom
		$messages.appendChild(this.#_message);

		scroll();
	}

	#handlePreview(event) {
		const inline = event.target.closest(".inline-file[data-id],.inline-svg[data-id]"),
			id = inline?.dataset?.id;

		if (!id) {
			return;
		}

		const file = this.#inline[id];

		if (!file) {
			notify(`Error: invalid file "${id}"`, "error");

			return;
		}

		if (event.target.classList.contains("download")) {
			download(file.name, "text/plain", file.content);

			return;
		}

		previewFile(file);
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

	#setupImage(img) {
		if (img.dataset.setup) {
			return;
		}

		const container = img.closest(".image-wrapper");

		if (!container) {
			return;
		}

		img.dataset.setup = "true";

		const infoBox = make("div", "image-info");

		container.appendChild(infoBox);

		const callback = () => {
			img.classList.add("loaded");

			const w = img.naturalWidth,
				h = img.naturalHeight;

			if (w > maxImageDimension || h > maxImageDimension) {
				const scale = maxImageDimension / Math.max(w, h),
					sw = Math.round(w * scale),
					sh = Math.round(h * scale);

				infoBox.textContent = `${sw}×${sh} (${w}×${h})`;
			} else {
				infoBox.textContent = `${w}×${h}`;
			}

			$messages.scrollTop += bHeight(img);
		};

		if (img.complete) {
			callback();
		} else {
			img.addEventListener("load", callback, {
				once: true,
			});
		}
	}

	#morph(from, to) {
		morphdom(from, to, {
			childrenOnly: true,
			onBeforeElUpdated: (fromEl, toEl) => {
				return !fromEl.isEqualNode || !fromEl.isEqualNode(toEl);
			},
			onBeforeNodeDiscarded: node => {
				if (node.classList?.contains("image-info")) {
					return false;
				}
			},
			onNodeAdded: node => {
				if (node.tagName === "IMG") {
					this.#setupImage(node);
				}
			},
		});
	}

	#patch(name, element, md, noScroll = false, after = null) {
		if (!element.firstChild) {
			const { html, files } = render(md);

			element.innerHTML = html;

			after?.();

			noScroll || scroll();

			this.#inline = files;

			element.querySelectorAll("img").forEach(img => {
				this.#setupImage(img);
			});

			return;
		}

		this.#pending[name] = md;

		if (this.#patching[name]) {
			return;
		}

		this.#patching[name] = true;

		schedule(() => {
			const { html, files } = render(this.#pending[name]);

			this.#patching[name] = false;

			this.#inline = files;

			this.#_diff.innerHTML = html;

			this.#morph(element, this.#_diff);

			after?.();

			this.#_diff.innerHTML = "";
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

				const image = this.#images[x];

				const _link = make("a", "image", "image-wrapper", `i-${x}`);

				_link.target = "_blank";
				_link.href = image;

				dataUrlFilename(image).then(name => {
					_link.download = name;
				});

				this.#_images.appendChild(_link);

				const _image = make("img");

				_image.style.aspectRatio = getDataUrlAspectRatio(image);

				_image.src = image;

				_link.appendChild(_image);

				this.#setupImage(_image);
			}

			this.#_message.classList.toggle("has-images", !!this.#images.length);
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
				_result.innerHTML = render(result ? wrapJSON(result) : "*processing*").html;

				this.#_tool.classList.toggle("invalid", !!invalid);

				this.#_tool.setAttribute("data-tool", name);
			} else {
				this.#_tool.removeAttribute("data-tool");
			}

			this.#_message.classList.toggle("has-tool", !!this.#tool);

			this.#updateToolHeight();
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

		if (!only || only === "time") {
			this.#_time.innerHTML = "";

			if (this.#time) {
				if (this.#ttft) {
					const ttft = make("span", "ttft-real");

					ttft.title = "Real time to first token";
					ttft.textContent = `${formatMilliseconds(this.#ttft * 1000)}`;

					this.#_time.appendChild(ttft);
				}

				const time = make("span");

				time.title = "Total time taken";
				time.textContent = formatMilliseconds(this.#time * 1000);

				this.#_time.appendChild(time);
			}
		}

		if (this.#error) {
			noScroll || scroll();

			updateScrollButton();

			return;
		}

		if (!only || only === "reasoning") {
			let reasoning = this.#reasoning || "";

			if (this.#reasoningType === "reasoning.summary") {
				reasoning = reasoning.replace(/\s*\*\*([^\n*]+)\*\*\s*/gm, (_match, title) => {
					return `\n\n### ${title}\n`;
				});
			}

			this.#patch("reasoning", this.#_reasoning, reasoning, noScroll, () => {
				this.#updateReasoningHeight();
			});

			this.#_message.classList.toggle("has-reasoning", !!reasoning);
		}

		if (!only || only === "text") {
			let text = this.#text;

			// Replace image placeholders with actual data URLs for display
			for (const [hash, dataUrl] of this.#inlineImages) {
				const regex = new RegExp(`!\\[([^\\]]*)\\]\\(${hash}\\.[^)]+\\)`, "g");
				text = text.replace(regex, `![$1](${dataUrl})`);
			}

			if (text && this.#tags.includes("json")) {
				text = `\`\`\`json\n${text}\n\`\`\``;
			}

			this.#patch("text", this.#_text, text, noScroll);

			this.#_message.classList.toggle("has-text", !!this.#text);
		}

		noScroll || scroll();

		updateScrollButton();
	}

	#save() {
		store("messages", messages.map(message => message.getData(true)).filter(Boolean));
	}

	isAssistant() {
		return this.#role === "assistant";
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

	getData(full = false, expandImages = false) {
		let text = this.#text;

		if (expandImages) {
			const resizePromises = [];

			for (const [hash, dataUrl] of this.#inlineImages) {
				resizePromises.push(resizeDataUrl(dataUrl).then(resized => ({ hash: hash, dataUrl: resized })));
			}

			return Promise.all(resizePromises).then(results => {
				for (const { hash, dataUrl: resizedUrl } of results) {
					const regex = new RegExp(`!\\[([^\\]]*)\\]\\(${hash}\\.[^)]+\\)`, "g");

					text = text.replace(regex, `![$1](${resizedUrl})`);
				}

				return this.#buildData(text, full);
			});
		}

		return this.#buildData(text, full);
	}

	#buildData(text, full) {
		const data = {
			role: this.#role,
			text: text,
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

			if (this.#reasoningType) {
				data.reasoningType = this.#reasoningType;
			}
		}

		if (this.#images.length) {
			data.images = this.#images;
		}

		// Store inline images for persistence (as array of entries)
		if (this.#inlineImages.size > 0 && full) {
			data.inlineImages = Array.from(this.#inlineImages.entries());
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

		if (this.#time && full) {
			data.time = this.#time;

			if (this.#ttft) {
				data.ttft = this.#ttft;
			}
		}

		if (this.#_message.classList.contains("collapsed") && full) {
			data.collapsed = true;
		}

		if (!data.error && !data.images?.length && !data.files?.length && !data.reasoning && !data.text && !data.tool) {
			return false;
		}

		return data;
	}

	setRole(role) {
		if (this.#role === role) {
			return;
		}

		const oldRole = this.#role;

		this.#role = role;

		this.#_message.classList.remove(oldRole);
		this.#_message.classList.add(role);

		const wrapper = this.#_message.querySelector(".role");

		if (wrapper) {
			wrapper.classList.remove(oldRole);
			wrapper.classList.add(role);
		}

		const attachBtn = this.#_message.querySelector(".attach");

		if (attachBtn) {
			attachBtn.classList.toggle("none", role !== "user");
		} else if (role === "user") {
			const _opts = this.#_message.querySelector(".options"),
				_optCopy = this.#_message.querySelector(".options .copy");

			if (_opts && _optCopy) {
				const _attach = make("button", "attach");

				_attach.title = "Add files to this message";
				_opts.insertBefore(_attach, _optCopy);

				_attach.addEventListener("click", () => {
					uploadToMessage(_attach, this);
				});
			}
		}

		const _optRetry = this.#_message.querySelector(".options .retry");

		if (_optRetry) {
			const _assistant = role === "assistant";

			_optRetry.title = _assistant ? "Delete message and messages after this one and try again" : "Delete messages after this one and try again";
		}

		this.#save();
	}

	setStatistics(statistics) {
		this.#statistics = statistics;

		this.#render("statistics");
		this.#save();
	}

	setTime(time, ttft, final = false) {
		this.#time = time;

		if (ttft && !this.#ttft) {
			this.#ttft = ttft;
		}

		this.#render("time");

		if (final) {
			this.#save();
		}
	}

	async loadGenerationData(generationID) {
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
		} catch (err) {
			console.error(err);
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
		if (!file.id) {
			file.id = uid();
		}

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

		this.#_message.classList.toggle("busy", state && state !== "editing");

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

	addInlineImage(hash, dataUrl) {
		this.#inlineImages.set(hash, dataUrl);
	}

	getEditTextarea() {
		return this.#_edit;
	}

	addReasoning(chunk) {
		this.#reasoning += chunk;

		this.#render("reasoning");
		this.#save();
	}

	setReasoningType(type) {
		this.#reasoningType = type;

		this.#render("reasoning");
		this.#save();
	}

	addText(text) {
		this.#text += text;

		this.#render("text");
		this.#save();
	}

	isEmpty() {
		if (this.#text.trim().length) {
			return false;
		}

		if (this.#images.length) {
			return false;
		}

		return !this.#tool;
	}

	setError(error) {
		if (typeof error === "object") {
			if ("Message" in error) {
				error = error.Message;
			} else {
				error = JSON.stringify(error);
			}
		}

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
			this.#_edit.value = this.#text;

			this.setState("editing");

			this.updateEditHeight();

			this.#_edit.focus();
		} else {
			const newText = this.#_edit.value;

			for (const [hash, _dataUrl] of this.#inlineImages) {
				const regex = new RegExp(`\\(${hash}\\.[^)]+\\)`);

				if (!regex.test(newText)) {
					this.#inlineImages.delete(hash);
				}
			}

			this.#text = newText;

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

		if (messages.length === 0) {
			chatTitle = false;

			updateTitle();
		} else {
			const userMessages = messages.filter(msg => msg.isUser());

			if (userMessages.length === 0) {
				chatTitle = false;

				updateTitle();
			}
		}
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
						data = unpack(packed);
					} catch (err) {
						console.warn("bad chunk data", packed);
						console.warn(err);
					}
				}

				buffer = buffer.slice(5 + length);

				if (type === "alive") {
					continue; // we are still alive
				}

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

		console.error(err);

		callback({
			type: "error",
			data: err.message,
		});
	} finally {
		callback(aborted ? "aborted" : "done");
	}
}

let abortCallback;

function abortNow() {
	if (!abortCallback) {
		return false;
	}

	abortCallback();

	return true;
}

async function buildRequest(noPush = false) {
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

	const opts = settings.enabled
		? {
				name: settings.name,
				prompt: settings.prompt,
			}
		: null;

	const expandedMessages = await Promise.all(messages.map(message => message.getData(false, true)));

	return {
		prompt: $prompt.value,
		model: $model.value,
		provider: $providerSorting.value,
		temperature: temperature,
		iterations: iterations,
		tools: {
			json: jsonMode,
			search: searchTool,
		},
		image: {
			resolution: $imageResolution.value,
			aspect: $imageAspect.value,
		},
		reasoning: {
			effort: effort,
			tokens: tokens || 0,
		},
		metadata: {
			timezone: timezone,
			platform: platform,
			settings: opts,
		},
		messages: expandedMessages.filter(Boolean),
	};
}

async function generate(cancel = false, noPush = false) {
	if (abortNow() && cancel) {
		return;
	}

	if (autoScrolling) {
		setFollowTail(true);
	}

	const body = await buildRequest(noPush);

	const controller = new AbortController();

	$chat.classList.add("completing");

	const hasUser = !!messages.find(msg => msg.isUser()),
		hasAssistant = !!messages.find(msg => msg.isAssistant());

	if (!chatTitle || (hasUser && !hasAssistant)) {
		refreshTitle();
	}

	let message, generationID, stopTimeout, timeInterval, started, receivedToken, hasContent;

	function startLoadingTimeout() {
		stopTimeout?.();

		if (!message) {
			return;
		}

		const msg = message,
			timeout = setTimeout(() => {
				msg.setLoading(true);
			}, 1500);

		stopTimeout = () => {
			stopTimeout = null;

			clearTimeout(timeout);

			msg?.setLoading(false);
		};
	}

	let aborted;

	function finish(error = false) {
		if (!message) {
			return;
		}

		const msg = message,
			genID = generationID;

		clearInterval(timeInterval);

		const took = Math.round((Date.now() - started) / 100) / 10;

		msg.setTime(took, false);

		msg.setState(false);

		msg.loadGenerationData(genID);

		refreshUsage();

		if (error || !hasContent) {
			setGenerationState("error");
		} else {
			resetGenerationState();
		}

		receivedToken = false;
		hasContent = false;

		message = null;
		generationID = null;
	}

	function start() {
		started = Date.now();
		hasContent = false;

		setGenerationState("waiting");

		message = new Message({
			role: "assistant",
		});

		message.setState("waiting");

		if (jsonMode) {
			message.addTag("json");
		}

		if (searchTool) {
			message.addTag("search");
		}

		timeInterval = setInterval(() => {
			if (!message) {
				return;
			}

			const took = Math.round((Date.now() - started) / 100) / 10;

			message.setTime(took, receivedToken ? took : false);
		}, 100);
	}

	abortCallback = () => {
		abortCallback = null;
		aborted = true;

		controller.abort();

		stopTimeout?.();

		finish();

		$chat.classList.remove("completing");

		if (!chatTitle && !titleController) {
			refreshTitle();
		}
	};

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
			if (aborted) {
				return;
			}

			if (chunk === "done") {
				abortCallback();

				return;
			}

			stopTimeout?.();

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
					receivedToken = true;

					setGenerationState("completing");

					message.setState("tooling");
					message.setTool(chunk.data);

					hasContent = !message.isEmpty();

					if (chunk.data?.done) {
						finish();
					} else {
						return; // prevent loading bar
					}

					break;
				case "image":
					receivedToken = true;

					setGenerationState("completing");

					message.addImage(chunk.data);

					hasContent = !message.isEmpty();

					break;
				case "reason":
					receivedToken = true;

					if (!hasContent) {
						setGenerationState("reasoning");
					}

					message.setState("reasoning");
					message.addReasoning(chunk.data);

					break;
				case "reason_type":
					receivedToken = true;

					message.setReasoningType(chunk.data);

					break;
				case "text":
					receivedToken = true;

					setGenerationState("completing");

					message.setState("receiving");
					message.addText(chunk.data);

					hasContent = !message.isEmpty();

					break;
				case "error":
					setGenerationState("error");

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

		if (!response.ok || !result?.title) {
			throw new Error(result?.error || response.statusText);
		}

		chatTitle = result.title;

		refreshUsage();
	} catch (err) {
		if (err.name === "AbortError") {
			return;
		}

		console.error(err);

		notify(err, "error");
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

let usageController;

async function refreshUsage() {
	usageController?.abort();

	const controller = new AbortController();

	usageController = controller;

	$total.classList.add("loading");

	try {
		const response = await fetch("/-/usage", {
				signal: controller.signal,
			}),
			data = await response.json();

		if (!data || data.error) {
			throw new Error(data?.error || response.statusText);
		}

		totalUsage = data;

		updateTotalUsage();
	} catch (err) {
		if (!controller.signal.aborted) {
			notify(`Failed to refresh usage: ${err.message}`, "error");

			usageController = null;
		}
	} finally {
		if (!controller.signal.aborted) {
			$total.classList.remove("loading");
		}
	}
}

async function loadData() {
	const [_, data] = await Promise.all([connectDB(), json("/-/data")]);

	setupCrossTabSync();

	settings.enabled = load("s-enabled", true);
	settings.name = load("s-name", "");
	settings.prompt = load("s-prompt", "");

	$sEnabled.checked = settings.enabled;
	$sName.value = settings.name;
	$sPrompt.value = settings.prompt;

	const personalizationCollapsed = load("personalization-collapsed", false);

	if (personalizationCollapsed) {
		$personalizationBody.classList.add("collapsed");
		$personalizationCollapse.classList.add("collapsed");
	}

	const sidebarOpen = load("sidebar-open", false);

	if (sidebarOpen) {
		$sidebar.classList.add("open");
		document.body.classList.add("sidebar-open");
	}

	updatePersonalizationVisualState();

	if (!data) {
		notify("Failed to load data.", "error", true);

		return;
	}

	// render version
	$version.innerHTML = `<a href="https://github.com/coalaura/whiskr" target="_blank">whiskr</a> ${data.version === "dev" ? "dev" : `<a href="https://github.com/coalaura/whiskr/releases/tag/${data.version}" target="_blank">${data.version}</a>`}`;

	// usage
	usageType = load("usage-type", "monthly");

	updateTotalUsage();

	// update search availability
	searchAvailable = data.config.search;

	// initialize floaters (unless disabled)
	if (!data.config.motion) {
		initFloaters();
	}

	// show login modal
	if (data.config.auth && !data.authenticated) {
		$authentication.classList.add("open");
	}

	// render models
	const favorites = load("model-favorites", []),
		modelTab = load("model-tab"),
		newTime = Math.round(Date.now() / 1000) - 2 * 7 * 24 * 60 * 60;

	$model.addEventListener("tab", event => {
		const tab = event.detail;

		store("model-tab", tab);
	});

	$model.addEventListener("favorite", event => {
		const newFavorites = event.detail;

		store("model-favorites", newFavorites);
	});

	fillSelect($model, data.models, (el, model) => {
		const separator = "─".repeat(24);

		el.title = [
			model.name,
			separator,
			`Tags:\t\t${model.tags?.join(", ") || "-"}`,
			`Created:\t\t${formatTimestamp(model.created)}`,
			`Pricing/1M:\t${formatMoney(model.pricing.input)} In | ${formatMoney(model.pricing.output)} Out`,
			model.pricing.image ? `Images/1K:\t${formatMoney(model.pricing.image * 1000)} Out` : null,
			separator,
			stripMarkdown(model.description),
		]
			.filter(Boolean)
			.join("\n");

		el.value = model.id;
		el.textContent = model.name;

		if ((model.created || 0) >= newTime) {
			el.dataset.new = "yes";
		}

		if (favorites.includes(model.id)) {
			el.dataset.favorite = "yes";
		}

		const maxPrice = Math.max(model.pricing.input, model.pricing.output);

		if (maxPrice >= 40) {
			el.dataset.classes = "super-expensive";
		} else if (maxPrice >= 30) {
			el.dataset.classes = "very-expensive";
		} else if (maxPrice >= 20) {
			el.dataset.classes = "expensive";
		} else if (maxPrice <= 0) {
			el.dataset.classes = "free";
		} else if (maxPrice <= 1) {
			el.dataset.classes = "very-cheap";
		} else if (maxPrice <= 5) {
			el.dataset.classes = "cheap";
		} else {
			el.dataset.classes = "normal";
		}

		el.dataset.classes += ",model";

		const tags = model.tags || [];

		el.dataset.tags = tags.join(",");

		if (tags.includes("image")) {
			if (data.config.images) {
				el.dataset.tabs = "images";
			} else {
				el.dataset.disabled = "yes";

				disabledModels.push(model.id);
			}
		}

		models[model.id] = model;
		modelList.push(model);
	});

	dropdown($model, 6, favorites, data.config.images ? ["images"] : [], ["image"]).switchTab(modelTab);

	// render prompts
	data.prompts.unshift({
		key: "",
		name: "No Prompt",
	});

	fillSelect($prompt, data.prompts, (el, prompt) => {
		el.value = prompt.key;
		el.title = prompt.description;
		el.textContent = prompt.name;

		promptList.push(prompt);
	});

	dropdown($prompt);

	// render saved chats
	renderSavedChats();
}

function clearMessages() {
	abortNow();

	while (messages.length) {
		messages[0].delete();
	}
}

function restore() {
	const resizedHeight = load("resized");

	if (resizedHeight) {
		$chat.style.height = `${resizedHeight}px`;
	}

	$message.value = load("message", "");
	$role.value = load("role", "user");
	$model.value = load("model", modelList.length ? modelList[0].id : "");
	$prompt.value = load("prompt", promptList.length ? promptList[0].key : "");
	$temperature.value = load("temperature", 0.85);
	$iterations.value = load("iterations", 3);
	$providerSorting.value = load("provider", "");
	$imageResolution.value = load("image-resolution", "1K");
	$imageAspect.value = load("image-aspect", "");
	$reasoningEffort.value = load("reasoning-effort", "medium");
	$reasoningTokens.value = load("reasoning-tokens", 1024);

	if (!modelList.find(model => model.id === $model.value && !disabledModels.includes(model.id))) {
		$model.value = modelList.length ? modelList[0].id : "";
	}

	$model.dispatchEvent(new Event("change"));

	const files = load("attachments", []);

	for (const file of files) {
		pushAttachment(file);
	}

	if (!jsonMode && load("json")) {
		$json.click();
	}

	if (!searchTool && load("search")) {
		$search.click();
	} else {
		$iterations.parentNode.classList.add("none");
	}

	if (!autoScrolling && load("scrolling")) {
		$scrolling.click();
	}

	load("messages", []).forEach(message => {
		new Message(message);
	});

	chatTitle = load("title");

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

	_name.addEventListener("click", () => {
		previewFile(file);
	});

	_name.addEventListener("auxclick", event => {
		if (event.button !== 1) {
			return;
		}

		previewFile(file);
	});

	// metadata overlay (size + tokens)
	const _meta = make("div", "tokens");

	// size line
	const _size = make("div");

	_size.textContent = formatBytes(new Blob([file.content]).size);

	_meta.appendChild(_size);

	// tokens line (if available)
	if ("tokens" in file && Number.isInteger(file.tokens)) {
		const _tokens = make("div");

		_tokens.textContent = `~${new Intl.NumberFormat("en-US").format(file.tokens)} tokens`;

		_meta.appendChild(_tokens);

		_meta.classList.add("has-tokens");
	}

	_file.appendChild(_meta);

	// remove button
	const _remove = make("button", "remove");

	_remove.title = "Remove attachment";

	_file.appendChild(_remove);

	_remove.addEventListener("click", () => {
		callback(_file);
	});

	return _file;
}

function pushAttachment(file, message = false) {
	file.id = uid();

	if (message) {
		message.addFile(file);

		return;
	}

	attachments.push(file);

	store("attachments", attachments);

	$attachments.appendChild(
		buildFileElement(file, el => {
			const index = attachments.findIndex(attachment => attachment.id === file.id);

			if (index === -1) {
				return;
			}

			attachments.splice(index, 1);

			store("attachments", attachments);

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

	store("attachments", []);
}

function pushMessage() {
	const text = $message.value.trim();

	if (!text && !attachments.length) {
		return false;
	}

	const usedImages = new Map(),
		imageRegex = /!\[.*?\]\(([a-f0-9]{8})\.[^)]+\)/g;

	let match;

	while ((match = imageRegex.exec(text)) !== null) {
		const hash = match[1];

		if (pendingImages.has(hash)) {
			usedImages.set(hash, pendingImages.get(hash));

			pendingImages.delete(hash);
		}
	}

	$message.value = "";
	store("message", "");

	const message = new Message({
		role: $role.value,
		text: text,
		files: attachments,
		inlineImages: usedImages,
	});

	clearAttachments();
	updateTitle();

	return message;
}

async function uploadToMessage(self, message = false) {
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
			}

			if (file.content.includes("\0")) {
				throw new Error("File is not a text file");
			}

			if (file.content.length > 4 * 1024 * 1024) {
				throw new Error("File is too big (max 4MB)");
			}
		},
		msg => notify(msg, "error")
	);

	if (!files.length) {
		return;
	}

	isUploading = true;

	self.classList.add("loading");

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
		pushAttachment(file, message);
	}

	self.classList.remove("loading");

	isUploading = false;
}

async function uploadImageInline() {
	if (isUploading) {
		return;
	}

	const input = document.createElement("input");

	input.type = "file";
	input.accept = "image/*";
	input.multiple = true;

	input.onchange = async () => {
		if (!input.files.length) {
			return;
		}

		isUploading = true;
		$upload.classList.add("loading");

		for (const file of input.files) {
			if (!file.type.startsWith("image/")) {
				continue;
			}

			const dataUrl = await readFileAsDataUrl(file);

			await insertImageIntoTextarea(dataUrl, $message);
		}

		$upload.classList.remove("loading");
		isUploading = false;
	};

	input.click();
}

function getChatData(name) {
	return {
		title: name?.trim?.() || chatTitle,
		message: $message.value,
		attachments: attachments,
		role: $role.value,
		model: $model.value,
		provider: $providerSorting.value,
		prompt: $prompt.value,
		temperature: $temperature.value,
		iterations: $iterations.value,
		image: {
			resolution: $imageResolution.value,
			aspect: $imageAspect.value,
		},
		reasoning: {
			effort: $reasoningEffort.value,
			tokens: $reasoningTokens.value,
		},
		json: jsonMode,
		search: searchTool,
		messages: messages.map(message => message.getData(true)).filter(Boolean),
		savedAt: Date.now(),
	};
}

function closeSidebar() {
	$sidebar.classList.remove("open");

	document.body.classList.remove("sidebar-open");

	store("sidebar-open", false);
}

function toggleSidebar() {
	$sidebar.classList.toggle("open");

	document.body.classList.toggle("sidebar-open");

	store("sidebar-open", $sidebar.classList.contains("open"));
}

function getSavedChats() {
	return load("saved-chats", []);
}

function saveChatToStorage(name, skipConfirm = false) {
	name = name?.trim();

	if (!name) {
		notify("Please enter a name for the chat", "warning");

		return false;
	}

	const chatData = getChatData(name),
		savedChats = getSavedChats(),
		existingIndex = savedChats.findIndex(chat => chat.name === name);

	if (existingIndex !== -1) {
		if (!skipConfirm && !confirm(`A chat named "${name}" already exists. Overwrite?`)) {
			return false;
		}

		savedChats[existingIndex] = {
			name: name,
			data: chatData,
		};
	} else {
		savedChats.push({
			name: name,
			data: chatData,
		});
	}

	store("saved-chats", savedChats);

	renderSavedChats();

	notify(`Chat "${name}" saved`, "success");

	return true;
}

function loadChatFromStorage(name) {
	const savedChats = getSavedChats(),
		savedChat = savedChats.find(chat => chat.name === name);

	if (!savedChat) {
		notify(`Chat "${name}" not found`, "error");

		return;
	}

	const data = savedChat.data;

	clearMessages();

	// restore all state
	chatTitle = data.title;

	store("title", data.title);
	store("message", data.message);
	store("attachments", data.attachments);
	store("role", data.role);
	store("model", data.model);
	store("prompt", data.prompt);
	store("temperature", data.temperature);
	store("iterations", data.iterations);
	store("provider", data.provider);
	store("image-resolution", data.image?.resolution);
	store("image-aspect", data.image?.aspect);
	store("reasoning-effort", data.reasoning?.effort);
	store("reasoning-tokens", data.reasoning?.tokens);
	store("json", data.json);
	store("search", data.search);
	store("messages", data.messages);

	restore();

	closeSidebar();

	notify(`Loaded chat "${name}"`, "success");
}

function deleteChatFromStorage(name) {
	const savedChats = getSavedChats(),
		filtered = savedChats.filter(chat => chat.name !== name);

	store("saved-chats", filtered);

	renderSavedChats();

	notify(`Deleted chat "${name}"`, "success");
}

function renderSavedChats() {
	const savedChats = getSavedChats();

	$savedChatsList.innerHTML = "";

	if (savedChats.length === 0) {
		const empty = make("div", "empty-state");

		empty.textContent = "No saved chats yet";

		$savedChatsList.appendChild(empty);

		return;
	}

	// sort by saved date, newest first
	const sorted = [...savedChats].sort((a, b) => (b.data.savedAt || 0) - (a.data.savedAt || 0));

	for (const chat of sorted) {
		// main item
		const item = make("div", "saved-chat-item");

		// info wrapper
		const info = make("div", "chat-info");

		// name
		const name = make("div", "chat-name");

		name.textContent = chat.name;

		info.appendChild(name);

		// meta
		const meta = make("div", "chat-date"),
			date = chat.data.savedAt ? new Date(chat.data.savedAt).toLocaleDateString() : "Unknown date",
			messageCount = chat.data.messages?.length || 0;

		meta.textContent = `${date} - ${messageCount} messages`;

		info.appendChild(meta);

		// actions wrapper
		const actions = make("div", "chat-actions");

		// load chat
		const loadBtn = make("button", "load-chat");

		loadBtn.title = "Load this chat";

		loadBtn.addEventListener("click", event => {
			event.stopPropagation();

			loadChatFromStorage(chat.name);
		});

		actions.appendChild(loadBtn);

		// overwrite chat
		const overwriteBtn = make("button", "overwrite-chat");

		overwriteBtn.title = "Overwrite with current chat";

		overwriteBtn.addEventListener("click", event => {
			event.stopPropagation();

			if (confirm(`Overwrite saved chat "${chat.name}" with current chat state?`)) {
				saveChatToStorage(chat.name, true);
			}
		});

		actions.appendChild(overwriteBtn);

		// delete chat
		const deleteBtn = make("button", "delete-chat");

		deleteBtn.title = "Delete this chat";

		deleteBtn.addEventListener("click", event => {
			event.stopPropagation();

			if (confirm(`Delete saved chat "${chat.name}"?`)) {
				deleteChatFromStorage(chat.name);
			}
		});

		actions.appendChild(deleteBtn);

		// append
		item.appendChild(info);
		item.appendChild(actions);

		item.addEventListener("click", () => {
			loadChatFromStorage(chat.name);
		});

		$savedChatsList.appendChild(item);
	}
}

$total.addEventListener("click", () => {
	switch (usageType) {
		case "total":
			usageType = "daily";

			break;
		case "monthly":
			usageType = "total";

			break;
		case "weekly":
			usageType = "monthly";

			break;
		case "daily":
			usageType = "weekly";

			break;
	}

	store("usage-type", usageType);

	updateTotalUsage();
});

$total.addEventListener("auxclick", event => {
	if (event.button !== 1) {
		return;
	}

	refreshUsage();
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

		store("resized", false);

		scroll(isAtBottom, true);

		return;
	}

	if (event.button !== 0) {
		return;
	}

	isResizing = true;
	scrollResize = isAtBottom;

	document.body.classList.add("resizing");
});

$role.addEventListener("change", () => {
	store("role", $role.value);
});

$model.addEventListener("change", () => {
	const model = $model.value,
		data = model ? models[model] : null,
		tags = data?.tags || [];

	store("model", model);

	if (tags.includes("reasoning")) {
		$reasoningEffort.parentNode.classList.remove("none");
		$reasoningTokens.parentNode.classList.toggle("none", !!$reasoningEffort.value);
	} else {
		$reasoningEffort.parentNode.classList.add("none");
		$reasoningTokens.parentNode.classList.add("none");
	}

	if (tags.includes("image")) {
		$imageResolution.parentNode.classList.remove("none");
		$imageAspect.parentNode.classList.remove("none");
	} else {
		$imageResolution.parentNode.classList.add("none");
		$imageAspect.parentNode.classList.add("none");
	}

	const hasJson = tags.includes("json"),
		hasSearch = searchAvailable && tags.includes("tools");

	$json.classList.toggle("none", !hasJson);
	$search.classList.toggle("none", !hasSearch);

	$search.parentNode.classList.toggle("none", !hasJson && !hasSearch);
	$iterations.parentNode.classList.toggle("none", !hasSearch || !searchTool);
});

$prompt.addEventListener("change", () => {
	store("prompt", $prompt.value);
});

$temperature.addEventListener("input", () => {
	const value = $temperature.value,
		temperature = parseFloat(value);

	store("temperature", value);

	$temperature.classList.toggle("invalid", Number.isNaN(temperature) || temperature < 0 || temperature > 2);
});

$iterations.addEventListener("input", () => {
	const value = $iterations.value,
		iterations = parseFloat(value);

	store("iterations", value);

	$iterations.classList.toggle("invalid", Number.isNaN(iterations) || iterations < 1 || iterations > 50);
});

$providerSorting.addEventListener("change", () => {
	store("provider", $providerSorting.value);
});

$imageResolution.addEventListener("change", () => {
	store("image-resolution", $imageResolution.value);
});

$imageAspect.addEventListener("change", () => {
	store("image-aspect", $imageAspect.value);
});

$reasoningEffort.addEventListener("change", () => {
	const effort = $reasoningEffort.value;

	store("reasoning-effort", effort);

	$reasoningTokens.parentNode.classList.toggle("none", !!effort);
});

$reasoningTokens.addEventListener("input", () => {
	const value = $reasoningTokens.value,
		tokens = parseInt(value, 10);

	store("reasoning-tokens", value);

	$reasoningTokens.classList.toggle("invalid", Number.isNaN(tokens) || tokens <= 0 || tokens > 1024 * 1024);
});

$json.addEventListener("click", () => {
	jsonMode = !jsonMode;

	store("json", jsonMode);

	$json.classList.toggle("on", jsonMode);
});

$search.addEventListener("click", () => {
	searchTool = !searchTool;

	store("search", searchTool);

	$search.classList.toggle("on", searchTool);

	$iterations.parentNode.classList.toggle("none", !searchTool);
});

$message.addEventListener("input", () => {
	store("message", $message.value);
});

$message.addEventListener("paste", async event => {
	const items = event.clipboardData?.items;

	if (!items) {
		return;
	}

	for (const item of items) {
		if (item.type.startsWith("image/")) {
			event.preventDefault();

			const file = item.getAsFile(),
				dataUrl = await readFileAsDataUrl(file);

			await insertImageIntoTextarea(dataUrl, $message);
		}
	}
});

$upload.addEventListener("click", event => {
	if (event.shiftKey) {
		uploadImageInline();
	} else {
		uploadToMessage($upload, false);
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

	chatTitle = false;

	updateTitle();
});

$sidebarTrigger.addEventListener("click", toggleSidebar);
$sidebarClose.addEventListener("click", closeSidebar);

$saveCurrentChat.addEventListener("click", () => {
	const name = prompt("Enter a name for this chat:", chatTitle || "New Chat");

	if (name) {
		saveChatToStorage(name);
	}
});

$export?.addEventListener("click", () => {
	const data = JSON.stringify(getChatData(false));

	download("chat.json", "application/json", data);
});

$import?.addEventListener("click", async () => {
	if (!modelList.length) {
		return;
	}

	const file = await selectFile(
			"application/json",
			false,
			selected => {
				selected.content = JSON.parse(selected.content);
			},
			msg => notify(msg, "error")
		),
		data = file?.content;

	if (!data) {
		return;
	}

	clearMessages();

	await Promise.all([
		store("title", data.title),
		store("message", data.message),
		store("attachments", data.attachments),
		store("role", data.role),
		store("model", data.model),
		store("prompt", data.prompt),
		store("temperature", data.temperature),
		store("iterations", data.iterations),
		store("image-resolution", data.image?.resolution),
		store("image-aspect", data.image?.aspect),
		store("reasoning-effort", data.reasoning?.effort),
		store("reasoning-tokens", data.reasoning?.tokens),
		store("json", data.json),
		store("search", data.search),
		store("messages", data.messages),
	]);

	restore();

	closeSidebar();
});

$dump.addEventListener("click", async () => {
	if (isDumping) {
		return;
	}

	isDumping = true;

	$dump.classList.add("loading");

	const body = await buildRequest(true);

	try {
		const response = await fetch("/-/dump", {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(body),
		});

		const dumped = await response.json();

		if (!response.ok) {
			throw new Error(dumped?.error || response.statusText);
		}

		download("request.json", "application/json", JSON.stringify(dumped.request, null, 4));
	} catch (err) {
		console.error(err);

		notify(err, "error");
	}

	$dump.classList.remove("loading");

	isDumping = false;
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

	store("scrolling", autoScrolling);
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
		console.error(err);

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

$sEnabled.addEventListener("change", () => {
	settings.enabled = $sEnabled.checked;

	store("s-enabled", settings.enabled);

	updatePersonalizationVisualState();
});

$sName.addEventListener("change", () => {
	settings.name = $sName.value.trim();

	store("s-name", settings.name);
});

$sPrompt.addEventListener("change", () => {
	settings.prompt = $sPrompt.value.trim();

	store("s-prompt", settings.prompt);
});

$message.addEventListener("keydown", event => {
	if (event.shiftKey) {
		return;
	}

	if (event.ctrlKey && event.key === "Enter") {
		$send.click();
	}
});

$personalizationHeader.addEventListener("click", event => {
	if (event.target.closest(".toggle-switch") || event.target.closest("input[type='checkbox']")) {
		return;
	}

	const isCollapsed = $personalizationBody.classList.toggle("collapsed");

	$personalizationCollapse.classList.toggle("collapsed", isCollapsed);

	store("personalization-collapsed", isCollapsed);
});

addEventListener("mousemove", event => {
	if (!isResizing) {
		return;
	}

	const total = window.innerHeight,
		height = clamp(window.innerHeight - event.clientY + (attachments.length ? 50 : 0), 200, total - 240);

	$chat.style.height = `${height}px`;

	store("resized", height);

	scroll(scrollResize, true);
});

addEventListener("mouseup", () => {
	isResizing = false;

	document.body.classList.remove("resizing");
});

addEventListener("keydown", event => {
	if (event.key === "Escape") {
		if ($sidebar.classList.contains("open")) {
			closeSidebar();
			return;
		}
	}

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

addEventListener("resize", () => {
	updateScrollButton();
});

document.body.classList.toggle("christmas", new Date().getMonth() === 11);

dropdown($role);
dropdown($providerSorting);
dropdown($imageResolution);
dropdown($imageAspect);
dropdown($reasoningEffort);

refreshUsage();

loadData().then(() => {
	restore();

	document.body.classList.remove("loading");

	document.body.classList.add("ready");
});
