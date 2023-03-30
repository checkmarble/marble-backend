package app

type Node interface {
	Returns() DataType
	Eval(Payload) interface{}
	Print(Payload) string
}

///////////////////////////////
// New
///////////////////////////////

// // what operators need
// type APIField string // "tx.amount" : no relationship, directly in payload
// type DBField string  // "tx.user.name" : with relationship, from DB
// type DBVariable struct {
// 	query  string            // "SELECT avg(amount) FROM tx WHERE user_id = $1 AND date >= NOW() - 7 DAYS" : query > STRONG LINK BT app and Repo
// 	params map[string]string // params[$1] = "tx.user_id" : parameters
// }
// type List []string

// // Interfaces
// type Operator interface {
// 	Needs() ([]APIField, []DBField, []DBVariable, []List)

// 	DescribeForFront() []byte // JSON representation of each operator > STRONG LINK BT app and API
// }

// type IntOperator interface {
// 	Operator
// 	Eval(map[APIField]any, map[DBField]any, map[DBVariable]any, []List) int
// }

// type BoolOperator interface {
// 	Operator
// 	Eval(map[APIField]any, map[DBField]any, map[DBVariable]any, []List) bool
// }

// type BoolEq struct{ Left, Right BoolOperator }

// func (eq BoolEq) Eval() bool {
// 	return eq.Left.Eval() == eq.Right.Eval()
// }

// tx.status = validated && date <= timestamp constant

// Dans le front:
// operator ID > affichage géré par le front
// operator ID > validation & definition des operandes dupliquée entre front et back

// Serialization BE x DB:
// chaque operateur a un nom/ID et une liste d'opérandes
// pour deserialiser: switch(operator.Id) case {implementation concrète}
