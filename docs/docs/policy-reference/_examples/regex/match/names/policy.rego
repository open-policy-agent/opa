package play

import rego.v1

name_pattern := `^(\p{L}+\s?)+\p{L}+$`

valid_name1 := regex.match(name_pattern, "Juan Pérez")

valid_name2 := regex.match(name_pattern, "张伟")

invalid_name1 := regex.match(name_pattern, "Juan ")

invalid_name2 := regex.match(name_pattern, "- 张伟")
