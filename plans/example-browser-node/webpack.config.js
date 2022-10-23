const path = require('path')
const webpack = require('webpack')
const source = path.resolve(__dirname, 'src')

module.exports = {
  context: __dirname,
  entry: './src/index.js',
  output: {
    path: path.resolve(__dirname, 'server', 'static', 'assets'),
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
  resolve: {
    extensions: ['.js'],
    fallback: {
      fs: false,
      os: require.resolve('os-browserify'),
      path: require.resolve('path-browserify'),
      https: require.resolve('https-browserify'),
      crypto: require.resolve('crypto-browserify'),
      http: require.resolve('stream-http'),
      zlib: require.resolve('browserify-zlib'),
      buffer: require.resolve('buffer/'),
      url: require.resolve('url/'),
      'assert/': require.resolve('assert/'),
      stream: require.resolve('stream-browserify')
    }
  },
  plugins: [
    new webpack.ProvidePlugin({
      Buffer: ['buffer', 'Buffer']
    }),
    new webpack.ProvidePlugin({
      process: 'process/browser'
    })
  ],
  mode: 'development'
}
