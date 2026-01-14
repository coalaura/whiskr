import { defineConfig } from "@rsbuild/core";

export default defineConfig({
	html: {
		template: "./preview/preview.html",
	},
	source: {
		entry: {
			preview: "./preview/preview.js",
		},
	},
	output: {
		distPath: {
			root: "internal",
			html: "./",
		},
		inlineScripts: true,
		inlineStyles: true,
		copy: [],
		cleanDistPath: false,
	},
});
