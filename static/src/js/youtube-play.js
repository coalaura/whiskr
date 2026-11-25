// Also included verbatim in standalone HTML exports.
document.addEventListener("click", event => {
	const button = event.target.closest?.("button.youtube-preview"),
		videoID = button?.parentElement?.dataset.youtubeId;

	if (!videoID || !/^[a-zA-Z0-9_-]{11}$/.test(videoID)) {
		return;
	}

	const player = document.createElement("iframe"),
		previewWidth = button.offsetWidth,
		previewHeight = button.offsetHeight;

	// Keep the image's layout contribution when replacing it with an iframe.
	button.parentElement.style.width = `${previewWidth}px`;
	player.style.aspectRatio = `${previewWidth} / ${previewHeight}`;

	player.className = "inline-youtube";
	player.title = button.getAttribute("aria-label");
	player.allowFullscreen = true;
	player.allow = "autoplay; encrypted-media; picture-in-picture; fullscreen";
	player.referrerPolicy = "strict-origin-when-cross-origin";
	player.src = `https://www.youtube-nocookie.com/embed/${videoID}?autoplay=1`;

	button.replaceWith(player);
	player.focus();
});

document.addEventListener("play", event => {
	const player = event.target;

	if (player.matches(".inline-media > audio, .inline-media > video")) {
		player.dataset.playback = "true";
	}
}, true);
