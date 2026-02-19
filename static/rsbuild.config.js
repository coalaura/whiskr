import { defineConfig } from "@rsbuild/core";

export default defineConfig({
	html: {
		template: "./src/index.html",
	},
	source: {
		entry: {
			index: "./src/js/chat.js",
		},
	},
	output: {
		distPath: {
			root: "dist",
			css: "css",
			js: "js",
		},
	},
});
