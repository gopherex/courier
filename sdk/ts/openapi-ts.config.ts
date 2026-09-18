import { defineConfig } from '@hey-api/openapi-ts';
export default defineConfig({input:'../../openapi/openapi.yaml',output:{path:'src/gen'},plugins:['@hey-api/client-fetch']});
