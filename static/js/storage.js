/** biome-ignore-all lint/correctness/noUnusedVariables: utility */

(() => {
	const DatabaseName = "whiskr",
		StorageName = "chat";

	function isNull(value) {
		return value === null || value === false || value === undefined;
	}

	class Database {
		#database;

		#scheduled = new Map();
		#writes = new Map();
		#cache = new Map();

		static async new() {
			const db = new Database();

			await db.#connect();
			await db.#load();

			return db;
		}

		#connect() {
			return new Promise((resolve, reject) => {
				const request = indexedDB.open(DatabaseName, 1);

				request.onerror = () => reject(request.error);

				request.onsuccess = () => {
					this.#database = request.result;

					resolve();
				};

				request.onupgradeneeded = event => {
					const db = event.target.result;

					if (db.objectStoreNames.contains(StorageName)) {
						return;
					}

					db.createObjectStore(StorageName);
				};
			});
		}

		#load() {
			return new Promise((resolve, reject) => {
				const transaction = this.#database.transaction(StorageName, "readonly"),
					store = transaction.objectStore(StorageName),
					request = store.openCursor();

				request.onerror = () => reject(request.error);

				let total = 0;

				request.onsuccess = event => {
					const cursor = event.target.result;

					if (cursor) {
						if (!isNull(cursor.value)) {
							this.#cache.set(cursor.key, cursor.value);

							total++;
						}

						cursor.continue();
					} else {
						console.info(`Loaded ${total} items from IndexedDB`);

						resolve();
					}
				};
			});
		}

		#write(key, retry) {
			if (this.#writes.has(key)) {
				if (retry) {
					this.#schedule(key);
				}

				return;
			}

			this.#writes.set(key, true);

			try {
				const transaction = this.#database.transaction(StorageName, "readwrite"),
					store = transaction.objectStore(StorageName);

				const value = this.#cache.get(key);

				if (isNull(value)) {
					store.delete(key);
				} else {
					store.put(value, key);
				}

				return new Promise((resolve, reject) => {
					transaction.oncomplete = () => resolve();
					transaction.onerror = () => reject(transaction.error);
				});
			} catch (error) {
				console.error(`Failed to write to IndexedDB: ${error}`);
			} finally {
				this.#writes.delete(key);
			}
		}

		#schedule(key) {
			if (this.#scheduled.has(key)) {
				clearTimeout(this.#scheduled.get(key));
			}

			const timeout = setTimeout(() => {
				this.#scheduled.delete(key);

				this.#write(key, true);
			}, 500);

			this.#scheduled.set(key, timeout);
		}

		store(key, value = false) {
			if (isNull(value)) {
				this.#cache.delete(key);
			} else {
				this.#cache.set(key, value);
			}

			this.#schedule(key);
		}

		load(key, fallback = false) {
			if (!this.#cache.has(key)) {
				return fallback;
			}

			return this.#cache.get(key);
		}
	}

	let db;

	window.connectDB = async () => {
		db = await Database.new();
	};

	window.storeValue = (key, value = false) => db.store(key, value);
	window.loadValue = (key, fallback = false) => db.load(key, fallback);
})();
