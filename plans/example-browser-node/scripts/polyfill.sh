#!/bin/bash

# polyfill setimmediate for winston in browser
sed "1s/^/require('setimmediate')\n\n/" src/index.js > src/index_polyfill.js
mv src/index_polyfill.js src/index.js
