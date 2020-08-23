# Godometer frontend

Basic usage during development

```bash
yarn
yarn dev
```

Or to build the deployment version

```bash
yarn build
```

The dev server will listen on port `8088` and proxy requests to `/api/*` to your local
`godoserv` instance on port `8080`. Be sure to adjust `rollup.config.js` if you want to
connect elsewhere.
