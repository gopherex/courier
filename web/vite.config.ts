import { defineConfig } from 'vite';
export default defineConfig({server:{proxy:{'/admin':'http://localhost:8080'}},build:{sourcemap:false}});
