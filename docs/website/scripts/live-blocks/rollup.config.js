import babel from '@rollup/plugin-babel'
import commonjs from '@rollup/plugin-commonjs'
import json from '@rollup/plugin-json'
import postcss from 'rollup-plugin-postcss'
import resolve from '@rollup/plugin-node-resolve'
import {terser} from 'rollup-plugin-terser'

import pkg from './package.json'

export default [
  // browser-friendly UMD build
  {
    input: 'src/web/index.js',
    output: {
      file: pkg.browser,
      format: 'umd',
    },
    plugins: [
      json(),
      resolve(),
      commonjs({
        include: ['node_modules/**'],
        sourceMap: false
      }),
      babel({// Taken from package.js, wasn't using plugins correctly otherwise
        'plugins': [
          [
            '@babel/plugin-proposal-decorators',
            {
              'legacy': true
            }
          ],
          '@babel/plugin-transform-react-jsx',
          '@babel/plugin-transform-regenerator'
        ],
        'presets': [
          '@babel/preset-env'
        ],
        exclude: ['node_modules/**'],
      }),
      postcss({
        extract: true
      }),
      terser(),
    ]
  }
]