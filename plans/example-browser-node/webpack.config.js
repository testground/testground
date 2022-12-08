const path = require('path')
const source = path.resolve(__dirname, 'src')

module.exports = {
  context: __dirname,
  entry: './src/index.js',
  output: {
    path: path.resolve(__dirname, 'runtime', 'server', 'static', 'assets'),
    filename: 'plan.bundle.js',
    library: '$',
    libraryTarget: 'umd'
  },
  module: {
    rules: [{
      test: /\.(js)$/,
      exclude: /node_modules/,
      include: source,
      use: 'babel-loader'
    }]
  },
  mode: 'development'
}
