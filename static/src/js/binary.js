function byte(binary, i) {
	return binary.charCodeAt(i);
}

function u16(binary, i) {
	return (byte(binary, i) << 8) | byte(binary, i + 1);
}

function u32(binary, i) {
	return (byte(binary, i) << 24) | (byte(binary, i + 1) << 16) | (byte(binary, i + 2) << 8) | byte(binary, i + 3);
}

function u16LE(binary, i) {
	return byte(binary, i) | (byte(binary, i + 1) << 8);
}

function u24LE(binary, i) {
	return byte(binary, i) | (byte(binary, i + 1) << 8) | (byte(binary, i + 2) << 16);
}

function u32LE(binary, i) {
	return byte(binary, i) | (byte(binary, i + 1) << 8) | (byte(binary, i + 2) << 16) | (byte(binary, i + 3) << 24);
}

export function getDataUrlAspectRatio(dataUrl) {
	const binary = atob(dataUrl.split(",")[1]);

	let width, height;

	if (byte(binary, 0) === 0x89 && byte(binary, 1) === 0x50) {
		// PNG: IHDR chunk at fixed position

		width = u32(binary, 16);
		height = u32(binary, 20);
	} else if (byte(binary, 0) === 0xff && byte(binary, 1) === 0xd8) {
		// JPEG: find SOF marker

		for (let i = 2; i < binary.length - 9; ) {
			if (byte(binary, i) !== 0xff) {
				i++;

				continue;
			}

			const marker = byte(binary, i + 1);

			if (marker >= 0xc0 && marker <= 0xc3) {
				height = u16(binary, i + 5);
				width = u16(binary, i + 7);

				break;
			}

			i += 2 + u16(binary, i + 2);
		}
	} else if (binary.slice(0, 4) === "RIFF" && binary.slice(8, 12) === "WEBP") {
		// WebP: RIFF container

		const fmt = binary.slice(12, 16);

		if (fmt === "VP8 ") {
			width = u16LE(binary, 26) & 0x3fff;
			height = u16LE(binary, 28) & 0x3fff;
		} else if (fmt === "VP8L") {
			const bits = u32LE(binary, 21);
			width = (bits & 0x3fff) + 1;
			height = ((bits >> 14) & 0x3fff) + 1;
		} else if (fmt === "VP8X") {
			width = u24LE(binary, 24) + 1;
			height = u24LE(binary, 27) + 1;
		}
	}

	return width && height ? width / height : null;
}
