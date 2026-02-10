import Dexie from "dexie";

const DatabaseName = "whiskr",
	TableName = "kv";

function isNull(value) {
	return value === "" || value === null || value === undefined;
}

class StorageDB {
	#database;
	#scheduled = new Map();
	#writes = new Map();
	#cache = new Map();

	async init() {
		this.#database = new Dexie(DatabaseName);
		this.#database.version(1).stores({
			[TableName]: "&key",
		});

		await this.#database.open();
		await this.#load();
	}

	async #load() {
		const rows = await this.#database.table(TableName).toArray();
		let total = 0;

		rows.forEach(row => {
			if (!isNull(row.value)) {
				this.#cache.set(row.key, row.value);
				total++;
			}
		});

		console.info(`Loaded ${total} items from Dexie`);
	}

	async #write(key, retry) {
		if (this.#writes.has(key)) {
			if (retry) {
				await this.#schedule(key);
			}

			return;
		}

		this.#writes.set(key, true);

		try {
			const value = this.#cache.get(key);

			if (isNull(value)) {
				await this.#database.table(TableName).delete(key);
			} else {
				await this.#database.table(TableName).put({
					key: key,
					value: value,
					updatedAt: Date.now(),
				});
			}
		} catch (error) {
			console.error(`Failed to write to Dexie: ${error}`);
		} finally {
			this.#writes.delete(key);
		}
	}

	#wait(ms) {
		return new Promise(resolve => setTimeout(resolve, ms));
	}

	async #schedule(key) {
		if (this.#scheduled.has(key)) {
			return;
		}

		this.#scheduled.set(key, true);

		await this.#wait(500);

		this.#scheduled.delete(key);

		await this.#write(key, true);
	}

	async store(key, value = false) {
		if (isNull(value)) {
			this.#cache.delete(key);
		} else {
			this.#cache.set(key, value);
		}

		await this.#schedule(key);
	}

	load(key, fallback = false) {
		if (!this.#cache.has(key)) {
			return fallback;
		}

		return this.#cache.get(key);
	}
}

let db;

export async function connectDB() {
	if (db) {
		return;
	}

	const newDB = new StorageDB();

	await newDB.init();

	db = newDB;
}

export function store(key, value = false) {
	if (!db) {
		return;
	}

	db.store(key, value);
}

export function load(key, fallback = false) {
	if (!db) {
		return fallback;
	}

	return db.load(key, fallback);
}
