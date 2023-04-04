package operators

type ValueFloat struct{ Value float64 }

func (vf ValueFloat) Eval(d DataAccessor) float64 { return vf.Value }
