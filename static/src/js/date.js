const timeRgx = /^(\d{1,2})(:?(\d{1,2})(:?(\d{1,2}))?)?( ?([ap]m))?$/im,
	relativeRgx = /^(in|[+-])?(( ?\d+ ?[a-z]+)+)( ?ago)?$/im,
	partialRelativeRgx = /( ?(\d+) ?([a-z]+))/gi;

function setRelativeTime(date, input) {
	const match = input.match(relativeRgx);

	if (!match) {
		throw new Error(`invalid time: ${input}`);
	}

	const prefix = match[1]?.toLowerCase(),
		suffix = match[4]?.toLowerCase();

	const isIn = prefix === "in" || prefix === "+",
		isAgo = suffix === "ago" || prefix === "-";

	if (isIn === isAgo) {
		throw new Error(`missing "in" or "ago": ${input}`);
	}

	const parts = [...match[2].matchAll(partialRelativeRgx)];

	if (!parts.length) {
		throw new Error(`incomplete relative time: ${input}`);
	}

	let total = 0,
		months = 0,
		years = 0,
		seen = {};

	for (const part of parts) {
		const val = parseInt(part[2], 10),
			unit = part[3].toLowerCase();

		let normalized;

		switch (unit) {
			case "y":
			case "yrs":
			case "year":
			case "years":
				normalized = "year(s)";

				years += val;

				break;

			case "mo":
			case "mth":
			case "month":
			case "months":
				normalized = "month(s)";

				months += val;

				break;

			case "d":
			case "day":
			case "days":
				normalized = "day(s)";

				total += val * 24 * 60 * 60 * 1000;

				break;

			case "h":
			case "hr":
			case "hrs":
			case "hour":
			case "hours":
				normalized = "hour(s)";

				total += val * 60 * 60 * 1000;

				break;

			case "m":
			case "min":
			case "mins":
			case "minute":
			case "minutes":
				normalized = "minute(s)";

				total += val * 60 * 1000;

				break;

			case "s":
			case "sec":
			case "secs":
			case "second":
			case "seconds":
				normalized = "second(s)";

				total += val * 1000;

				break;

			default:
				throw new Error(`invalid time unit: ${unit}`);
		}

		if (seen[normalized]) {
			throw new Error(`duplicate ${normalized}`);
		}

		seen[normalized] = true;
	}

	if (years > 0) {
		date.setFullYear(date.getFullYear() + years);
	}

	if (months > 0) {
		date.setMonth(date.getMonth() + months);
	}

	if (total > 0) {
		date.setTime(date.getTime() + total);
	}
}

function setTime(date, input) {
	const match = input.match(timeRgx);

	if (!match) {
		setRelativeTime(date, input);

		return;
	}

	const sign = match[7]?.toLowerCase();

	let hour = parseInt(match[1], 10);

	if (sign) {
		if (hour < 1 || hour > 12) {
			throw new Error(`invalid hour (1-12): ${match[1]}`);
		}

		if (sign === "pm") {
			if (hour !== 12) {
				hour += 12;
			}
		} else {
			if (hour === 12) {
				hour = 0;
			}
		}
	} else {
		if (hour < 0 || hour > 23) {
			throw new Error(`invalid hour (0-23): ${match[1]}`);
		}
	}

	if (!match[3]) {
		date.setHours(hour, 0, 0, 0);

		return;
	}

	const minute = parseInt(match[3], 10);

	if (minute < 0 || minute > 59) {
		throw new Error(`invalid minute (0-59): ${match[3]}`);
	}

	if (!match[5]) {
		date.setHours(hour, minute, 0, 0);

		return;
	}

	const second = parseInt(match[5], 10);

	if (second < 0 || second > 59) {
		throw new Error(`invalid second (0-59): ${match[5]}`);
	}

	date.setHours(hour, minute, second, 0);
}

function asTimestamp(date) {
	return Math.round(date.getTime() / 1000);
}

export function parseDateTime(input) {
	if (!input) {
		return false;
	}

	input = input.trim();

	const date = new Date(input);

	if (!Number.isNaN(date.getTime())) {
		return asTimestamp(date);
	}

	const now = new Date();

	setTime(now, input);

	return asTimestamp(now);
}
