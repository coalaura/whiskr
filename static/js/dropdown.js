(() => {
	class Dropdown {
		#_select;
		#_dropdown;
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

		#maxTags = false;
		#search = false;
		#selected = false;
		#options = [];

		constructor(el, maxTags = false, favorites = false) {
			this.#_select = el;

			this.#maxTags = maxTags;
			this.#search = "searchable" in el.dataset;

			this.#_select.querySelectorAll("option").forEach(option => {
				const classes = option.dataset.classes?.trim(),
					tags = option.dataset.tags?.trim(),
					isFavorite = !!option.dataset.favorite,
					isNew = !!option.dataset.new;

				this.#options.push({
					value: option.value,
					label: option.textContent,

					title: option.title || "",
					classes: classes ? classes.split(",") : [],
					tags: tags ? tags.split(",") : [],
					favorite: isFavorite,
					new: isNew,

					search: searchable(option.textContent),
				});
			});

			this.#build(favorites);

			if (this.#options.length) {
				this.#set(this.#options[0].value);
			}
		}

		#build(favorites) {
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
			this.#_dropdown = make("div", "dropdown", favorites ? "has-tabs" : "no-tabs", this.#options.length >= 7 ? "full-height" : "");

			// selected item
			this.#_selected = make("div", "selected");

			this.#_selected.addEventListener("click", () => {
				this.#_dropdown.classList.toggle("open");

				const selection = this.#options[this.#selected];

				selection.el.scrollIntoView({
					behavior: "smooth",
					block: "nearest",
					inline: "nearest",
				});

				this.#_search.focus();
			});

			this.#_dropdown.appendChild(this.#_selected);

			// content
			const _content = make("div", "cont");

			this.#_dropdown.appendChild(_content);

			// tabs wrapper
			const _tabs = make("div", "tabs");

			_content.appendChild(_tabs);

			// option wrapper
			const _options = make("div", "opts");

			_content.appendChild(_options);

			// default tab
			this.#all.label = make("div", "tab-title", "active");

			this.#all.label.textContent = "All";

			_tabs.appendChild(this.#all.label);

			this.#all.count = make("sup", "count");

			this.#all.count.textContent = this.#options.length;

			this.#all.label.appendChild(this.#all.count);

			this.#all.label.addEventListener("click", () => {
				if (!favorites) {
					return;
				}

				this.switchTab(false);
			});

			this.#all.container = make("div", "tab", "active");

			_options.appendChild(this.#all.container);

			// favorites tab
			if (favorites) {
				this.#favorites.label = make("div", "tab-title");

				this.#favorites.label.textContent = "Favorites";

				_tabs.appendChild(this.#favorites.label);

				this.#favorites.count = make("sup", "count");

				this.#favorites.count.textContent = "0";

				this.#favorites.label.appendChild(this.#favorites.count);

				this.#favorites.label.addEventListener("click", () => {
					this.switchTab(true);
				});

				this.#favorites.container = make("div", "tab");

				_options.appendChild(this.#favorites.container);
			}

			// options
			for (const option of this.#options) {
				// option wrapper
				const _opt = make("div", "opt");

				_opt.title = option.title || "";

				_opt.classList.add(...option.classes);

				_opt.addEventListener("click", () => {
					this.#_select.value = option.value;

					this.#_dropdown.classList.remove("open");
				});

				// option label
				const _label = make("div", "label");

				_opt.appendChild(_label);

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

				// option tags (optional)
				const tags = option.tags;

				if (option.tags.length) {
					const _tags = make("div", "tags");

					_tags.title = `${this.#maxTags ? `${tags.length}/${this.#maxTags}: ` : ""}${tags.join(", ")}`;

					if (this.#maxTags && tags.length >= this.#maxTags) {
						const _all = make("div", "tag", "all");

						_tags.appendChild(_all);
					} else {
						for (const tag of tags) {
							const _tag = make("div", "tag", tag);

							_tags.appendChild(_tag);
						}
					}

					_opt.appendChild(_tags);
				}

				// add to options
				this.#all.container.appendChild(_opt);

				option.el = _opt;

				// handle favorite
				if (favorites) {
					if (option.favorite) {
						this.#makeFavorite(option, true);
					}

					_opt.addEventListener("auxclick", event => {
						if (event.button !== 1) {
							return;
						}

						this.#makeFavorite(option);
					});
				}
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

		#render() {
			if (this.#selected === false) {
				this.#_selected.title = "";
				this.#_selected.innerHTML = "";

				return;
			}

			for (const key in this.#options) {
				const option = this.#options[key];

				option.el.classList.remove("active");
				option.clone?.classList?.remove("active");
			}

			const selection = this.#options[this.#selected];

			selection.el.classList.add("active");
			selection.clone?.classList?.add("active");

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

		switchTab(favorites) {
			this.#all.label.classList.toggle("active", !favorites);
			this.#all.container.classList.toggle("active", !favorites);

			this.#favorites.label.classList.toggle("active", favorites);
			this.#favorites.container.classList.toggle("active", favorites);

			this.#trigger("tab", favorites ? "favorites" : "all");
		}

		#makeFavorite(option, force = false) {
			function remove() {
				option.el.classList.remove("favorite");

				if (option.clone) {
					option.clone.remove();

					option.clone = null;
				}
			}

			option.favorite = !option.favorite || force;

			this.#favorites.count.textContent = this.#options.filter(opt => opt.favorite).length;

			if (!force) {
				this.#trigger("favorite", {
					value: option.value,
					favorite: option.favorite,
				});
			}

			if (!option.favorite) {
				remove();

				return;
			}

			option.el.classList.add("favorite");

			option.clone = option.el.cloneNode(true);

			this.#favorites.container.appendChild(option.clone);

			option.clone.addEventListener("click", () => {
				this.#_select.value = option.value;

				this.#_dropdown.classList.remove("open");
			});

			option.clone.addEventListener("auxclick", event => {
				if (event.button !== 1) {
					return;
				}

				remove();
			});
		}

		#filter() {
			if (!this.#_search) {
				return;
			}

			const query = searchable(this.#_search.value);

			for (const option of this.#options) {
				if (query && !option.search.includes(query)) {
					option.el.classList.add("filtered");
					option.clone?.classList?.add("filtered");
				} else {
					option.el.classList.remove("filtered");
					option.clone?.classList?.remove("filtered");
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

	window.dropdown = (el, maxTags = false, favorites = false) => new Dropdown(el, maxTags, favorites);
})();
