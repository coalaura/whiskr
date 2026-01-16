import { make } from "./lib.js";

const states = {
	idle: null,
	waiting: "#8087a2",
	reasoning: "#eed49f",
	completing: "#a6da95",
	error: "#ed8796",
};

let currentState = "idle",
	originalFavicon,
	canvas,
	ctx,
	faviconLink;

function initFavicon() {
	faviconLink = document.querySelector(`link[rel="icon"]`);

	originalFavicon = faviconLink.href;

	canvas = make("canvas");

	canvas.width = 32;
	canvas.height = 32;

	ctx = canvas.getContext("2d");

	addEventListener("focus", () => {
		if (currentState === "error") {
			resetGenerationState();
		}
	});
}

function drawFavicon(state) {
	const color = states[state];

	if (!color) {
		if (originalFavicon) {
			faviconLink.href = originalFavicon;
		}

		return;
	}

	const img = new Image();

	img.crossOrigin = "anonymous";

	img.onload = () => {
		ctx.clearRect(0, 0, 32, 32);
		ctx.drawImage(img, 0, 0, 32, 32);

		ctx.beginPath();
		ctx.arc(24, 24, 7, 0, Math.PI * 2);
		ctx.fillStyle = "#1e2030"; // Dark background
		ctx.fill();

		ctx.beginPath();
		ctx.arc(24, 24, 5, 0, Math.PI * 2);
		ctx.fillStyle = color;
		ctx.fill();

		faviconLink.href = canvas.toDataURL("image/png");
	};

	img.onerror = () => {
		ctx.clearRect(0, 0, 32, 32);

		ctx.beginPath();
		ctx.arc(16, 16, 14, 0, Math.PI * 2);
		ctx.fillStyle = "#1e2030";
		ctx.fill();

		ctx.beginPath();
		ctx.arc(16, 16, 10, 0, Math.PI * 2);
		ctx.fillStyle = color;
		ctx.fill();

		faviconLink.href = canvas.toDataURL("image/png");
	};

	img.src = originalFavicon || "";
}

export function setGenerationState(newState) {
	if (newState === "error" && document.hasFocus()) {
		resetGenerationState();

		return;
	}

	currentState = newState;

	drawFavicon(currentState);
}

export function resetGenerationState() {
	currentState = "idle";

	drawFavicon("idle");
}

initFavicon();
