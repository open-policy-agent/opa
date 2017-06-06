require './lib/modular-scale'

Gem::Specification.new do |s|
  s.name        = 'modular-scale'
  s.version     = ModularScale::VERSION
  s.date        = ModularScale::DATE
  s.authors     = ['Scott Kellum', 'Mason Wendell', 'Adam Stacoviak']
  s.email       = ['scott@scottkellum.com', 'mason@thecodingdesigner.com', 'adam@stacoviak.com']
  s.homepage    = 'http://modularscale.com'
  s.license     = 'MIT'

  s.summary     = 'Modular scale calculator built into your Sass.'
  s.description = 'A modular scale is a list of values that share the same relationship. These
values are often used to size type and create a sense of harmony in a design. Proportions within
modular scales are all around us from the spacing of the joints on our fingers to branches on
trees. These natural proportions have been used since the time of the ancient Greeks in
architecture and design and can be a tremendously helpful tool to leverage for web designers.'

  s.files       = Dir['lib/**/*'] + Dir['stylesheets/**/*']
  s.extra_rdoc_files = ['changelog.md', 'license.md', 'readme.md']

  s.required_rubygems_version = '>= 1.3.6'

  s.add_runtime_dependency 'compass', '>= 0.12.0'
end
