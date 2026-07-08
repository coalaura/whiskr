import { defineConfig } from "@rsbuild/core";

export default defineConfig(({ command }) => {
	const isProd = command === "build";

	return {
		dev: {
			hmr: false,
			liveReload: false,
		},
		html: {
			template: "./src/index.html",
			minify: isProd,
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
			cleanDistPath: true,
			sourcemap: !isProd,
			minify: isProd
				? {
						js: { compress: { drop_console: true } },
						css: true,
					}
				: false,
			chunkSplit: {
				strategy: "split-by-size",
				minSize: 40 * 1024,
			},
		},
	};
});
