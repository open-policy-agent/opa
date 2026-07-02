package coding.tools

deny contains $"Tool {tc.tool} is not allowed" if {
	some tc in input.tool_calls
	tc.tool in _disallowed_tools
}

deny contains $"WebFetch can only load from HTTPS URLs" if {
	some tc in input.tool_calls
	tc.tool == "WebFetch"
	not startswith(tc.params.url, "https://")
}

deny contains $"WebSearch cannot load more than {_max_results} results" if {
	some tc in input.tool_calls
	tc.tool == "WebSearch"
	tc.params.num_results > _max_results
}

deny contains $"Tool timeout cannot be more than 10s" if {
	some tc in input.tool_calls
	tc.params.timeout > 10000
}

_disallowed_tools := {"Bash", "Write", "Edit"}

_max_results := 10
