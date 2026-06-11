package evaluate

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	lua "github.com/yuin/gopher-lua"
)

type LuaSandbox struct {
	state *lua.LState
}

func NewLuaSandbox() *LuaSandbox {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})

	libs := map[string]lua.LGFunction{
		lua.MathLibName:   lua.OpenMath,
		lua.StringLibName: lua.OpenString,
	}

	for name, lib := range libs {
		L.Push(L.NewFunction(lib))
		L.Push(lua.LString(name))
		L.Call(1, 0)
	}

	s := &LuaSandbox{state: L}

	s.ImportGlobalFunctions("assert", "error", "print", "tostring", "tonumber", "type", "pairs", "ipairs")
	s.ImportFunctions("os", "time", "date")

	return s
}

func (s *LuaSandbox) ImportFunctions(mod string, fns ...string) {
	LT := lua.NewState()
	fullMod := LT.FindTable(LT.Get(lua.GlobalsIndex).(*lua.LTable), mod, 0).(*lua.LTable)

	safeMod := s.state.NewTable()

	for _, fn := range fns {
		safeMod.RawSetString(fn, fullMod.RawGetString(fn))
	}

	s.state.SetGlobal(mod, safeMod)

	marbleMod := s.state.NewTable()
	marbleMod.RawSetString("debug", s.state.NewFunction(LuaDebug))

	s.state.SetGlobal("marble", marbleMod)
}

func (s *LuaSandbox) ImportGlobalFunctions(fns ...string) {
	LT := lua.NewState()

	fullMod := LT.Get(lua.GlobalsIndex).(*lua.LTable)
	safeMod := s.state.Get(lua.GlobalsIndex).(*lua.LTable)

	for _, fn := range fns {
		safeMod.RawSetString(fn, fullMod.RawGetString(fn))
	}
}

func (s *LuaSandbox) SetClientObject(attrs map[string]any) {
	if attrs == nil {
		s.state.SetGlobal("input", s.state.NewTable())
	}

	s.state.SetGlobal("input", s.mapToTable(attrs))
}

func (s *LuaSandbox) SetPivotObject(attrs map[string]any) {
	if attrs == nil {
		s.state.SetGlobal("client", s.state.NewTable())
	}

	s.state.SetGlobal("client", s.mapToTable(attrs))
}

func (s *LuaSandbox) Run(code string) (any, error) {
	if err := s.state.DoString(code); err != nil {
		return nil, err
	}

	if s.state.GetTop() == 0 {
		return nil, errors.New("no output")
	}

	ret := s.state.Get(-1)

	if ret == lua.LNil {
		return nil, nil
	}

	switch ret.Type() {
	case lua.LTBool:
		return bool(ret.(lua.LBool)), nil
	case lua.LTNumber:
		return float64(ret.(lua.LNumber)), nil
	case lua.LTString:
		return string(ret.(lua.LString)), nil
	default:
		return nil, errors.New("invalid return type")
	}
}

func LuaDebug(s *lua.LState) int {
	v := s.CheckAny(1)

	fmt.Printf("%#v\n", v)

	return 0
}

func (s *LuaSandbox) mapToTable(m map[string]any) *lua.LTable {
	t := s.state.NewTable()

	for k, v := range m {
		k = strings.Replace(k, `"`, "", -1)

		switch v := v.(type) {
		case string:
			t.RawSetString(k, lua.LString(v))
		case float64:
			t.RawSetString(k, lua.LNumber(v))
		case int:
			t.RawSetString(k, lua.LNumber(v))
		case bool:
			t.RawSetString(k, lua.LBool(v))
		case map[string]any:
			t.RawSetString(k, s.mapToTable(v))
		}
	}

	return t
}
