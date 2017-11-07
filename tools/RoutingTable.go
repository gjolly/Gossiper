package tools

type RoutingTable struct {
	table map[string]string
}

func newRoutingTable() *RoutingTable{
	return &RoutingTable{make(map[string]string, 0)}
}

func (r *RoutingTable) add(key string, value string){
	r.add(key, value)
}