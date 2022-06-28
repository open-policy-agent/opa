package wasm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/internal/wasmdebug"
)

// NewHostModule is defined internally for use in WASI tests and to keep the code size in the root directory small.
func NewHostModule(
	moduleName string,
	nameToGoFunc map[string]interface{},
	nameToMemory map[string]*Memory,
	nameToGlobal map[string]*Global,
	enabledFeatures Features,
) (m *Module, err error) {
	if moduleName != "" {
		m = &Module{NameSection: &NameSection{ModuleName: moduleName}}
	} else {
		m = &Module{}
	}

	funcCount := uint32(len(nameToGoFunc))
	memoryCount := uint32(len(nameToMemory))
	globalCount := uint32(len(nameToGlobal))
	exportCount := funcCount + memoryCount + globalCount
	if exportCount > 0 {
		m.ExportSection = make([]*Export, 0, exportCount)
	}

	// Check name collision as exports cannot collide on names, regardless of type.
	for name := range nameToGoFunc {
		if _, ok := nameToMemory[name]; ok {
			return nil, fmt.Errorf("func[%s] exports the same name as a memory", name)
		}
		if _, ok := nameToGlobal[name]; ok {
			return nil, fmt.Errorf("func[%s] exports the same name as a global", name)
		}
	}
	for name := range nameToMemory {
		if _, ok := nameToGlobal[name]; ok {
			return nil, fmt.Errorf("memory[%s] exports the same name as a global", name)
		}
	}

	if funcCount > 0 {
		if err = addFuncs(m, nameToGoFunc, enabledFeatures); err != nil {
			return
		}
	}

	if memoryCount > 0 {
		if err = addMemory(m, nameToMemory); err != nil {
			return
		}
	}

	// TODO: we can use enabledFeatures to fail early on things like mutable globals (once supported)
	if globalCount > 0 {
		if err = addGlobals(m, nameToGlobal); err != nil {
			return
		}
	}

	// Assins the ModuleID by calculating sha256 on inputs as host modules do not have `wasm` to hash.
	m.AssignModuleID([]byte(fmt.Sprintf("%s:%v:%v:%v:%v",
		moduleName, nameToGoFunc, nameToMemory, nameToGlobal, enabledFeatures)))
	return
}

func (m *Module) IsHostModule() bool {
	return len(m.HostFunctionSection) > 0
}

func addFuncs(m *Module, nameToGoFunc map[string]interface{}, enabledFeatures Features) error {
	funcCount := uint32(len(nameToGoFunc))
	funcNames := make([]string, 0, funcCount)
	if m.NameSection == nil {
		m.NameSection = &NameSection{}
	}
	m.NameSection.FunctionNames = make([]*NameAssoc, 0, funcCount)
	m.FunctionSection = make([]Index, 0, funcCount)
	m.HostFunctionSection = make([]*reflect.Value, 0, funcCount)

	// Sort names for consistent iteration
	for k := range nameToGoFunc {
		funcNames = append(funcNames, k)
	}
	sort.Strings(funcNames)

	for idx := Index(0); idx < funcCount; idx++ {
		name := funcNames[idx]
		fn := reflect.ValueOf(nameToGoFunc[name])
		_, functionType, err := getFunctionType(&fn, enabledFeatures)
		if err != nil {
			return fmt.Errorf("func[%s] %w", name, err)
		}

		m.FunctionSection = append(m.FunctionSection, m.maybeAddType(functionType))
		m.HostFunctionSection = append(m.HostFunctionSection, &fn)
		m.ExportSection = append(m.ExportSection, &Export{Type: ExternTypeFunc, Name: name, Index: idx})
		m.NameSection.FunctionNames = append(m.NameSection.FunctionNames, &NameAssoc{Index: idx, Name: name})
	}
	return nil
}

func addMemory(m *Module, nameToMemory map[string]*Memory) error {
	memoryCount := uint32(len(nameToMemory))

	// Only one memory can be defined or imported
	if memoryCount > 1 {
		memoryNames := make([]string, 0, memoryCount)
		for k := range nameToMemory {
			memoryNames = append(memoryNames, k)
		}
		sort.Strings(memoryNames) // For consistent error messages
		return fmt.Errorf("only one memory is allowed, but configured: %s", strings.Join(memoryNames, ", "))
	}

	// Find the memory name to export.
	var name string
	for k, v := range nameToMemory {
		name = k
		if v.Min > v.Max {
			return fmt.Errorf("memory[%s] min %d pages (%s) > max %d pages (%s)", name, v.Min, PagesToUnitOfBytes(v.Min), v.Max, PagesToUnitOfBytes(v.Max))
		}
		m.MemorySection = v
	}

	m.ExportSection = append(m.ExportSection, &Export{Type: ExternTypeMemory, Name: name, Index: 0})
	return nil
}

func addGlobals(m *Module, globals map[string]*Global) error {
	globalCount := len(globals)
	m.GlobalSection = make([]*Global, 0, globalCount)

	globalNames := make([]string, 0, globalCount)
	for name := range globals {
		globalNames = append(globalNames, name)
	}
	sort.Strings(globalNames) // For consistent iteration order

	for i, name := range globalNames {
		m.GlobalSection = append(m.GlobalSection, globals[name])
		m.ExportSection = append(m.ExportSection, &Export{Type: ExternTypeGlobal, Name: name, Index: Index(i)})
	}
	return nil
}

func (m *Module) maybeAddType(ft *FunctionType) Index {
	for i, t := range m.TypeSection {
		if t.EqualsSignature(ft.Params, ft.Results) {
			return Index(i)
		}
	}

	result := m.SectionElementCount(SectionIDType)
	m.TypeSection = append(m.TypeSection, ft)
	return result
}

func (m *Module) buildHostFunctions(
	moduleName string,
	functionListenerFactory experimental.FunctionListenerFactory,
) (functions []*FunctionInstance) {
	// ModuleBuilder has no imports, which means the FunctionSection index is the same as the position in the function
	// index namespace. Also, it ensures every function has a name. That's why there is less error checking here.
	var functionNames = m.NameSection.FunctionNames
	for idx, typeIndex := range m.FunctionSection {
		fn := m.HostFunctionSection[idx]
		f := &FunctionInstance{
			Kind:   kind(fn.Type()),
			Type:   m.TypeSection[typeIndex],
			GoFunc: fn,
			Idx:    Index(idx),
		}
		name := functionNames[f.Idx].Name
		f.moduleName = moduleName
		f.DebugName = wasmdebug.FuncName(moduleName, name, f.Idx)
		f.name = name
		// TODO: add parameter names for host functions (vararg strings that must match arity with param length)
		f.exportNames = []string{name}
		if functionListenerFactory != nil {
			f.FunctionListener = functionListenerFactory.NewListener(f)
		}
		functions = append(functions, f)
	}
	return
}
