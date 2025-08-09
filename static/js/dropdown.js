(() => {
	class Dropdown {
		#_select;
		#_dropdown;
		#_selected;

		#selected = false;
		#options = [];

		constructor(el) {
			this.#_select = el;

			this.#_select.querySelectorAll("option").forEach((option) => {
				this.#options.push({
					value: option.value,
					label: option.textContent.trim(),
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

			const descriptor = Object.getOwnPropertyDescriptor(
				HTMLSelectElement.prototype,
				"value",
			);

			Object.defineProperty(this.#_select, "value", {
				get: () => {
					return descriptor.get.call(this.#_select);
				},
				set: (value) => {
					descriptor.set.call(this.#_select, value);

					this.#set(value);
				},
			});

			// dropdown
			this.#_dropdown = make("div", "dropdown");

			this.#_dropdown.addEventListener("click", () => {
				this.#_dropdown.classList.add("open");
			});

			// selected item
			this.#_selected = make("div", "selected");

			this.#_dropdown.appendChild(this.#_selected);

			// option wrapper
			const _options = make("div", "options");

			this.#_dropdown.appendChild(_options);

			// options
			for (const option of this.#options) {
				const _opt = make("div", "option");

				_opt.textContent = option.label;

				_opt.addEventListener("click", () => {
					this.#set(option.value);
				});

				option.el = _opt;
			}

			// add to dom
			this.#_select.after(this.#_dropdown);

			this.#render();
		}

		#render() {
			if (this.#selected === false) {
				this.#_selected.textContent = "";

				return;
			}

			const selection = this.#options[this.#selected];

			this.#_selected.textContent = selection.label;
		}

		#set(value) {
			console.log("value", value);

			const index = this.#options.findIndex((option) => option.value === value);

			if (this.#selected === index) {
				return;
			}

			this.#selected = index !== -1 ? index : false;

			this.#render();
		}
	}

	document.body.addEventListener("click", (event) => {
		const clicked = event.target.closest(".dropdown");

		document.querySelectorAll(".dropdown").forEach((element) => {
			if (element === clicked) {
				return;
			}

			element.classList.remove("open");
		});
	});

	window.dropdown = (el) => new Dropdown(el);
})();
