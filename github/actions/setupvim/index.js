"use strict";

const spawn = require("child_process").spawn;

async function run() {
	var args = Array.prototype.slice.call(arguments);
	const cmd = spawn(args[0], args.slice(1), {
		stdio: "inherit",
		cwd: __dirname
	});
	const exitCode = await new Promise((resolve, reject) => {
		cmd.on("close", resolve);
	});
	if (exitCode != 0) {
		process.exit(exitCode);
	}
}

(async function() {
	const path = require("path");
	await run("go", "run", ".");
})();
