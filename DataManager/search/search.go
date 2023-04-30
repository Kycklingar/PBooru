package search

type Eval interface {
	Eval(b Writer)
}

//func NewJoinWrapper(table Table) *JoinWrapper {
//	return &JoinWrapper{
//		table: table,
//	}
//}
//
//type JoinWrapper struct {
//	table      Table
//	conditions Conditions
//}
//
//func (w *JoinWrapper) On(cond Condition) Logic {
//	w.conditions = append(w.conditions, cond)
//	return w
//}
//
//func (w *JoinWrapper) And(cond Condition) Logic {
//	return w.On(And(cond))
//}
//
//func (w *JoinWrapper) Or(cond Condition) Logic {
//	return w.On(Or(cond))
//}
