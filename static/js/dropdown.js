(() => {
	class Dropdown {
		#_select;
		#_dropdown;
		#_selected;
		#_search;

		#maxTags = false;
		#search = false;
		#selected = false;
		#options = [];

		constructor(el, maxTags = false) {
			this.#_select = el;

			this.#maxTags = maxTags;
			this.#search = "searchable" in el.dataset;

			this.#_select.querySelectorAll("option").forEach(option => {
				const tags = option.dataset.tags?.trim(),
					isNew = !!option.dataset.new;

				this.#options.push({
					value: option.value,
					label: option.textContent,

					title: option.title || "",
					tags: tags ? tags.split(",") : [],
					new: isNew,

					search: searchable(option.textContent),
				});
			});

			this.#build();

			if (this.#options.length) {
				this.#set(this.#options[0].value);
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
			this.#_dropdown = make("div", "dropdown");

			// selected item
			this.#_selected = make("div", "selected");

			this.#_selected.addEventListener("click", () => {
				this.#_dropdown.classList.toggle("open");

				const selection = this.#options[this.#selected];

				selection.el.scrollIntoView();
			});

			this.#_dropdown.appendChild(this.#_selected);

			// content
			const _content = make("div", "cont");

			this.#_dropdown.appendChild(_content);

			// option wrapper
			const _options = make("div", "opts");

			_content.appendChild(_options);

			// options
			for (const option of this.#options) {
				// option wrapper
				const _opt = make("div", "opt");

				_opt.title = option.title || "";

				_opt.addEventListener("click", () => {
					this.#_select.value = option.value;

					this.#_dropdown.classList.remove("open");
				});

				// option label
				const _label = make("div", "label");

				_label.textContent = option.label;

				_opt.appendChild(_label);

				// new tag
				if (option.new) {
					const _new = make("sup", "new");

					_new.textContent = "new";
					_new.title = "Less than 2 weeks old";

					_label.appendChild(_new);
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
				_options.appendChild(_opt);

				option.el = _opt;
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
			}

			const selection = this.#options[this.#selected];

			selection.el.classList.add("active");

			this.#_selected.classList.toggle("all-tags", selection.tags.length >= this.#maxTags);

			this.#_selected.title = selection.title || this.#_select.title;
			this.#_selected.innerHTML = selection.el.innerHTML;

			this.#_dropdown.setAttribute("data-value", selection.value);
		}

		#filter() {
			if (!this.#_search) {
				return;
			}

			const query = searchable(this.#_search.value);

			for (const option of this.#options) {
				if (query && !option.search.includes(query)) {
					option.el.classList.add("filtered");
				} else {
					option.el.classList.remove("filtered");
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

	window.dropdown = (el, maxTags = false) => new Dropdown(el, maxTags);
})();
