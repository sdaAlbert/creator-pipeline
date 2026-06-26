import { copyFile, mkdir, rm } from 'node:fs/promises';

await rm('dist', { recursive: true, force: true });
await mkdir('dist/src', { recursive: true });
await copyFile('index.html', 'dist/index.html');
await copyFile('src/main.js', 'dist/src/main.js');
await copyFile('src/styles.css', 'dist/src/styles.css');
console.log('Built static CreatorScript Studio to dist/');
