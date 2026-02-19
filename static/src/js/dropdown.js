import { make } from "./lib.js";

class Dropdown {
	#_select;
	#_dropdown;
	#_options;
	#_selected;
	#_search;

	#all = {
		label: false,
		count: false,
		container: false,
	};
	#favorites = {
		label: false,
		count: false,
		container: false,
	};
	#tabData = {};

	#maxTags = false;
	#search = false;
	#selected = false;
	#options = [];
	#tabs = [];

	#activeTab = "all";
	#tabScroll = {};

	#favoriteOrder = [];
	#favoritesEnabled = false;

	#dragState = {
		draggedOption: false,
		dropIndicator: false,
		container: false,
	};

	constructor(el, maxTags = false, favorites = false, tabs = []) {
		this.#_select = el;

		this.#maxTags = maxTags;
		this.#search = "searchable" in el.dataset;
		this.#tabs = Array.isArray(tabs) ? tabs : [];

		this.#favoritesEnabled = Array.isArray(favorites);
		this.#favoriteOrder = Array.isArray(favorites) ? [...favorites] : [];

		this.#_select.querySelectorAll("option").forEach(option => {
			const classes = option.dataset.classes?.trim(),
				tags = option.dataset.tags?.trim(),
				isFavorite = !!option.dataset.favorite,
				isDisabled = !!option.dataset.disabled,
				isNew = !!option.dataset.new,
				tabData = option.dataset.tabs?.trim();

			const allowedTabs = this.#tabs.length ? new Set(this.#tabs) : null;

			const optionTabs = tabData
				? tabData
						.split(",")
						.map(t => t.trim())
						.filter(Boolean)
						.filter(t => !allowedTabs || allowedTabs.has(t))
				: [];

			this.#options.push({
				value: option.value,
				label: option.textContent,

				title: option.title || "",
				classes: classes ? classes.split(",") : [],
				icon: option.dataset.icon,
				tags: tags ? tags.split(",") : [],
				favorite: isFavorite,
				disabled: isDisabled,
				new: isNew,
				tabs: optionTabs,

				search: searchable(option.textContent),
			});
		});

		this.#build();

		if (this.#options.length) {
			this.#set(this.#_select.value || this.#options[0].value);
		}
	}

	#build() {
		// prepare and hide original select
		this.#_select.style.display = "none";

		const descriptor = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value");

		Object.defineProperty(this.#_select, "value", {
			get: () => {
				return descriptor.get.call(this.#_select);
			},
			set: value => {
				descriptor.set.call(this.#_select, value);

				this.#_select.dispatchEvent(new Event("change"));

				this.#set(value);
			},
		});

		// dropdown
		this.#_dropdown = make("div", "dropdown", this.#favoritesEnabled || this.#tabs.length ? "has-tabs" : "no-tabs", this.#options.length >= 7 ? "full-height" : "");

		// selected item
		this.#_selected = make("div", "selected");

		this.#_selected.addEventListener("click", () => {
			const willOpen = !this.#_dropdown.classList.contains("open");

			if (willOpen) {
				const rect = this.#_dropdown.getBoundingClientRect();

				if (rect.top < 250 && rect.top < window.innerHeight - rect.bottom) {
					this.#_dropdown.classList.add("open-down");
				} else {
					this.#_dropdown.classList.remove("open-down");
				}
			}

			this.#_dropdown.classList.toggle("open");

			const selection = this.#options[this.#selected];

			selection.el.scrollIntoView({
				behavior: "smooth",
				block: "nearest",
				inline: "nearest",
			});

			this.#_search?.focus();
		});

		this.#_dropdown.appendChild(this.#_selected);

		// content
		const _content = make("div", "cont");

		this.#_dropdown.appendChild(_content);

		// tabs wrapper
		const _tabs = make("div", "tabs");

		_content.appendChild(_tabs);

		// option wrapper
		this.#_options = make("div", "opts");

		_content.appendChild(this.#_options);

		// default tab
		this.#all.label = make("div", "tab-title", "active");

		this.#all.label.textContent = "All";

		_tabs.appendChild(this.#all.label);

		this.#all.count = make("sup", "count");

		this.#all.count.textContent = this.#options.length;

		this.#all.label.appendChild(this.#all.count);

		this.#all.label.addEventListener("click", () => {
			if (!this.#favoritesEnabled) {
				return;
			}

			this.switchTab("all");
		});

		this.#all.container = make("div", "tab", "active");

		this.#_options.appendChild(this.#all.container);

		// custom tabs
		for (const tab of this.#tabs) {
			const label = make("div", "tab-title");

			label.textContent = tab.charAt(0).toUpperCase() + tab.slice(1);

			_tabs.appendChild(label);

			const count = make("sup", "count");

			count.textContent = this.#options.filter(opt => opt.tabs.includes(tab)).length;

			label.appendChild(count);

			label.addEventListener("click", () => {
				this.switchTab(tab);
			});

			const container = make("div", "tab");

			this.#_options.appendChild(container);

			this.#tabData[tab] = {
				label: label,
				count: count,
				container: container,
			};
		}

		// favorites tab
		if (this.#favoritesEnabled) {
			this.#favorites.label = make("div", "tab-title");

			this.#favorites.label.textContent = "Favorites";

			_tabs.appendChild(this.#favorites.label);

			this.#favorites.count = make("sup", "count");

			this.#favorites.count.textContent = "0";

			this.#favorites.label.appendChild(this.#favorites.count);

			this.#favorites.label.addEventListener("click", () => {
				this.switchTab("favorites");
			});

			this.#favorites.container = make("div", "tab");

			this.#favorites.container.classList.add("favorites-container");

			this.#_options.appendChild(this.#favorites.container);

			this.#setupFavoritesDragAndDrop();
		}

		// options
		for (const option of this.#options) {
			// option wrapper
			const _opt = make("div", "opt");

			_opt.title = option.title || "";

			_opt.classList.add(...option.classes);

			if (option.disabled) {
				_opt.classList.add("disabled");
			}

			_opt.addEventListener("click", () => {
				if (option.disabled) {
					return;
				}

				this.#_select.value = option.value;

				this.#_dropdown.classList.remove("open");
			});

			// option label
			const _label = make("div", "label");

			_opt.appendChild(_label);

			// icon (optional)
			if (option.icon) {
				const _icon = make("div", "icon");

				_icon.style.setProperty("background-image", `url(${option.icon})`);

				_label.appendChild(_icon);
			}

			const _span = make("span");

			_span.textContent = option.label;

			_label.appendChild(_span);

			// new tag
			if (option.new) {
				const _new = make("sup", "new");

				_new.textContent = "new";
				_new.title = "Less than 2 weeks old";

				_span.appendChild(_new);
			}

			// right tags (optional)
			if (option.tags.length) {
				const _tags = make("div", "tags");

				_tags.title = `${this.#maxTags ? `${option.tags.length}/${this.#maxTags}: ` : ""}${option.tags.join(", ")}`;

				if (this.#maxTags && option.tags.length >= this.#maxTags) {
					const _all = make("div", "tag", "all");

					_tags.appendChild(_all);
				} else {
					for (const tag of option.tags) {
						const _tag = make("div", "tag", tag);

						_tags.appendChild(_tag);
					}
				}

				_opt.appendChild(_tags);
			}

			// add to options (all)
			this.#all.container.appendChild(_opt);

			option.el = _opt;
			option.clones = {};

			// add to custom tabs
			for (const tab of option.tabs) {
				const tabMeta = this.#tabData[tab];

				if (!tabMeta) {
					continue;
				}

				const clone = option.el.cloneNode(true);

				tabMeta.container.appendChild(clone);

				clone.addEventListener("click", () => {
					if (option.disabled) {
						return;
					}

					this.#_select.value = option.value;

					this.#_dropdown.classList.remove("open");
				});

				clone.addEventListener("auxclick", event => {
					if (event.button !== 1) {
						return;
					}

					this.#makeFavorite(option);
				});

				option.clones[tab] = clone;
			}

			// handle favorite
			if (this.#favoritesEnabled) {
				if (this.#favoriteOrder.includes(option.value)) {
					option.favorite = true;
				}

				_opt.addEventListener("auxclick", event => {
					if (event.button !== 1) {
						return;
					}

					this.#makeFavorite(option);
				});
			}
		}

		// render favorites in order
		if (this.#favoritesEnabled) {
			for (const favId of this.#favoriteOrder) {
				const option = this.#options.find(opt => opt.value === favId);

				if (option?.favorite) {
					this.#createFavoriteClone(option, true);
				}
			}

			this.#updateFavoritesCount();
		}

		// live search (if enabled)
		if (this.#search) {
			this.#_search = make("input", "search");

			this.#_search.type = "text";
			this.#_search.placeholder = "Search...";

			this.#_search.addEventListener("input", () => {
				this.#filter();
			});

			this.#_search.addEventListener("keydown", event => {
				if (event.key !== "Escape") {
					return;
				}

				if (this.#_search.value) {
					this.#_search.value = "";

					this.#_search.dispatchEvent(new Event("input"));

					return;
				}

				this.#_dropdown.classList.remove("open");
			});

			_content.appendChild(this.#_search);
		}

		// add to dom
		this.#_select.after(this.#_dropdown);

		this.#render();
	}

	#setupFavoritesDragAndDrop() {
		const container = this.#favorites.container;

		this.#dragState.container = container;

		// drop indicator
		this.#dragState.dropIndicator = document.createElement("div");

		this.#dragState.dropIndicator.className = "drop-indicator";

		container.addEventListener("dragover", event => {
			event.preventDefault();

			this.#handleDragOver(event.clientY);
		});

		document.addEventListener("dragover", this.#handleDocumentDragOver);

		container.addEventListener("drop", event => {
			event.preventDefault();

			this.#handleDrop();
		});

		container.addEventListener("dragleave", event => {
			if (!container.contains(event.relatedTarget)) {
				this.#dragState.dropIndicator.remove();
			}
		});
	}

	#handleDocumentDragOver = event => {
		if (!this.#dragState.draggedOption) {
			return;
		}

		const container = this.#dragState.container,
			rect = container.getBoundingClientRect();

		if (event.clientY < rect.top) {
			event.preventDefault();

			const firstOpt = container.querySelector(".opt");

			if (firstOpt) {
				container.insertBefore(this.#dragState.dropIndicator, firstOpt);
			} else {
				container.appendChild(this.#dragState.dropIndicator);
			}
		} else if (event.clientY > rect.bottom) {
			event.preventDefault();

			container.appendChild(this.#dragState.dropIndicator);
		}
	};

	#handleDragOver(clientY) {
		const container = this.#dragState.container,
			dropIndicator = this.#dragState.dropIndicator,
			afterElement = this.#getDragAfterElement(container, clientY);

		if (afterElement) {
			afterElement.before(dropIndicator);
		} else {
			container.appendChild(dropIndicator);
		}
	}

	#handleDrop() {
		const container = this.#dragState.container,
			dropIndicator = this.#dragState.dropIndicator,
			draggable = container.querySelector(".dragging");

		if (draggable && dropIndicator.parentNode) {
			dropIndicator.before(draggable);

			this.#updateFavoriteOrderFromDOM();
		}
	}

	#getDragAfterElement(container, y) {
		const draggableElements = [...container.querySelectorAll(".opt:not(.dragging)")];

		return draggableElements.reduce(
			(closest, child) => {
				const box = child.getBoundingClientRect(),
					offset = y - box.top - box.height / 2;

				if (offset < 0 && offset > closest.offset) {
					return {
						offset: offset,
						element: child,
					};
				}

				return closest;
			},
			{
				offset: Number.NEGATIVE_INFINITY,
			}
		).element;
	}

	#updateFavoriteOrderFromDOM() {
		const newOrder = [];

		this.#favorites.container.querySelectorAll(".opt").forEach(el => {
			for (const option of this.#options) {
				if (option.favoriteClone === el) {
					newOrder.push(option.value);

					break;
				}
			}
		});

		this.#favoriteOrder = newOrder;

		this.#trigger("favorite", this.#favoriteOrder);
	}

	#updateFavoritesCount() {
		if (this.#favorites.count) {
			this.#favorites.count.textContent = this.#favoriteOrder.length;
		}
	}

	#createFavoriteClone(option, silent = false) {
		if (option.favoriteClone) {
			option.favoriteClone.remove();
		}

		option.el.classList.add("favorite");

		for (const tab in option.clones) {
			option.clones[tab].classList.add("favorite");
		}

		option.favoriteClone = option.el.cloneNode(true);

		option.favoriteClone.setAttribute("draggable", "true");
		option.favoriteClone.classList.add("favorite-item");

		option.favoriteClone.addEventListener("dragstart", event => {
			event.dataTransfer.setData("text/plain", option.value);

			event.dataTransfer.effectAllowed = "move";

			option.favoriteClone.classList.add("dragging");

			this.#dragState.draggedOption = option;
		});

		option.favoriteClone.addEventListener("dragend", () => {
			option.favoriteClone.classList.remove("dragging");

			this.#dragState.draggedOption = false;

			const indicator = this.#dragState.dropIndicator;

			if (indicator?.parentNode) {
				indicator.remove();
			}
		});

		// click to select
		option.favoriteClone.addEventListener("click", () => {
			if (option.disabled) {
				return;
			}

			this.#_select.value = option.value;

			this.#_dropdown.classList.remove("open");
		});

		// middle click to remove
		option.favoriteClone.addEventListener("auxclick", event => {
			if (event.button !== 1) {
				return;
			}

			this.#makeFavorite(option);
		});

		// Insert in correct position based on order
		const currentIndex = this.#favoriteOrder.indexOf(option.value),
			nextFavId = this.#favoriteOrder[currentIndex + 1];

		if (nextFavId) {
			const nextOption = this.#options.find(opt => opt.value === nextFavId);

			if (nextOption?.favoriteClone) {
				this.#favorites.container.insertBefore(option.favoriteClone, nextOption.favoriteClone);
			} else {
				this.#favorites.container.appendChild(option.favoriteClone);
			}
		} else {
			this.#favorites.container.appendChild(option.favoriteClone);
		}

		if (!silent) {
			this.#updateFavoritesCount();

			this.#trigger("favorite", this.#favoriteOrder);
		}
	}

	#render() {
		if (this.#selected === false) {
			this.#_selected.title = "";
			this.#_selected.innerHTML = "";

			return;
		}

		for (const key in this.#options) {
			const option = this.#options[key];

			option.el.classList.remove("active");
			option.favoriteClone?.classList?.remove("active");

			for (const tab in option.clones) {
				option.clones[tab].classList.remove("active");
			}
		}

		const selection = this.#options[this.#selected];

		selection.el.classList.add("active");
		selection.favoriteClone?.classList?.add("active");

		for (const tab in selection.clones) {
			selection.clones[tab].classList.add("active");
		}

		this.#_selected.classList.toggle("all-tags", selection.tags.length >= this.#maxTags);

		this.#_selected.title = selection.title || this.#_select.title;
		this.#_selected.innerHTML = selection.el.innerHTML;

		this.#_dropdown.setAttribute("data-value", selection.value);
	}

	#trigger(event, data) {
		this.#_select.dispatchEvent(
			new CustomEvent(event, {
				detail: data,
				bubbles: true,
			})
		);
	}

	switchTab(tab = "all") {
		if (this.#activeTab) {
			const current = this.#_options;

			this.#tabScroll[this.#activeTab] = current.scrollTop;
		}

		const isAll = tab === "all",
			isFav = tab === "favorites";

		this.#all.label.classList.toggle("active", isAll);
		this.#all.container.classList.toggle("active", isAll);

		if (this.#favorites.label) {
			this.#favorites.label.classList.toggle("active", isFav);
			this.#favorites.container.classList.toggle("active", isFav);
		}

		for (const tabName in this.#tabData) {
			const tabMeta = this.#tabData[tabName],
				active = tab === tabName;

			tabMeta.label.classList.toggle("active", active);
			tabMeta.container.classList.toggle("active", active);
		}

		this.#activeTab = tab;

		const saved = this.#tabScroll[this.#activeTab];

		if (typeof saved === "number") {
			this.#_options.scrollTop = saved;
		} else {
			const selection = this.#options[this.#selected];

			let el = null;

			if (selection) {
				if (tab === "all") {
					el = selection.el;
				} else if (tab === "favorites") {
					el = selection.favoriteClone;
				} else {
					el = selection.clones?.[tab];
				}
			}

			if (el) {
				el.scrollIntoView({
					behavior: "instant",
					block: "nearest",
					inline: "nearest",
				});
			} else {
				this.#_options.scrollTop = 0;
			}
		}

		this.#trigger("tab", tab);
	}

	#makeFavorite(option) {
		option.favorite = !option.favorite;

		if (!option.favorite) {
			const idx = this.#favoriteOrder.indexOf(option.value);

			if (idx > -1) {
				this.#favoriteOrder.splice(idx, 1);
			}

			option.el.classList.remove("favorite");

			for (const tab in option.clones) {
				option.clones[tab].classList.remove("favorite");
			}

			if (option.favoriteClone) {
				option.favoriteClone.remove();

				option.favoriteClone = null;
			}

			this.#updateFavoritesCount();

			this.#trigger("favorite", this.#favoriteOrder);

			return;
		}

		if (!this.#favoriteOrder.includes(option.value)) {
			this.#favoriteOrder.unshift(option.value);
		}

		this.#createFavoriteClone(option);
	}

	#filter() {
		if (!this.#_search) {
			return;
		}

		const query = searchable(this.#_search.value);

		for (const option of this.#options) {
			if (query && !option.search.includes(query)) {
				option.el.classList.add("filtered");
				option.favoriteClone?.classList?.add("filtered");

				for (const tab in option.clones) {
					option.clones[tab].classList.add("filtered");
				}
			} else {
				option.el.classList.remove("filtered");
				option.favoriteClone?.classList?.remove("filtered");

				for (const tab in option.clones) {
					option.clones[tab].classList.remove("filtered");
				}
			}
		}
	}

	#set(value) {
		const index = this.#options.findIndex(option => option.value === value);

		if (this.#selected === index) {
			return;
		}

		this.#selected = index !== -1 ? index : false;

		this.#render();
	}
}

function searchable(text) {
	// lowercase
	text = text.toLowerCase();

	// only alpha-num
	text = text.replace(/[^\w]/g, "");

	return text.trim();
}

document.body.addEventListener("click", event => {
	const clicked = event.target.closest(".dropdown");

	document.querySelectorAll(".dropdown").forEach(element => {
		if (element === clicked) {
			return;
		}

		element.classList.remove("open");
	});
});

export function dropdown(el, maxTags = false, favorites = false, tabs = []) {
	return new Dropdown(el, maxTags, favorites, tabs);
}
