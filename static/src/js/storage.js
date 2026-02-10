import Dexie from "dexie";
import "dexie-observable";

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
	#listeners = new Set();
	#localSource = null;
	#lastWrite = new Map();

	async init() {
		this.#database = new Dexie(DatabaseName);
		this.#database.version(1).stores({
			[TableName]: "&key",
		});

		await this.#database.open();

		this.#localSource = this.#resolveLocalSource();

		this.#database.on("changes", changes => this.#handleChanges(changes));

		await this.#load();
	}

	#resolveLocalSource() {
		return this.#database?._localSyncNode?.id || this.#database?._localSyncNode || this.#database?._localSyncNodeId || this.#database?._localSyncNodeID || null;
	}

	#emitChange(change) {
		for (const listener of this.#listeners) {
			listener(change);
		}
	}

	#handleChanges(changes) {
		this.#localSource ||= this.#resolveLocalSource();

		for (const change of changes) {
			if (change.table !== TableName) {
				continue;
			}

			const key = change.key,
				value = change.obj?.value ?? null,
				updatedAt = change.obj?.updatedAt ?? null;

			let isLocal = this.#localSource && change.source === this.#localSource;

			if (!isLocal && key && this.#lastWrite.has(key)) {
				const age = Date.now() - this.#lastWrite.get(key);

				if (age < 1500) {
					isLocal = true;
				}
			}

			this.#emitChange({
				key: key,
				value: value,
				updatedAt: updatedAt,
				type: change.type,
				isLocal: !!isLocal,
			});
		}
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

		this.#lastWrite.set(key, Date.now());

		await this.#schedule(key);
	}

	async refresh(keys = []) {
		if (!keys.length) {
			return new Map();
		}

		const table = this.#database.table(TableName),
			results = new Map();

		for (const key of keys) {
			const row = await table.get(key);

			if (row && !isNull(row.value)) {
				this.#cache.set(key, row.value);

				results.set(key, row.value);
			} else {
				this.#cache.delete(key);

				results.set(key, null);
			}
		}

		return results;
	}

	onChange(listener) {
		this.#listeners.add(listener);

		return () => this.#listeners.delete(listener);
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

export function onChange(listener) {
	if (!db) {
		return () => {};
	}

	return db.onChange(listener);
}

export async function refresh(keys = []) {
	if (!db) {
		return new Map();
	}

	return db.refresh(keys);
}
