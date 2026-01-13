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
	dev: {
		client: {
			port: 3000,
		},
	},
	server: {
		port: 3000,
	},
	output: {
		distPath: {
			root: "dist",
			css: "css",
			js: "js",
		},
	},
});
