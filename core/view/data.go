package view

type Data map[string]any

func (d Data) Set(key string, value any) Data {
	d[key] = value
	return d
}

func (d Data) SetDefault(key string, value any) Data {
	if _, exists := d[key]; !exists {
		d[key] = value
	}
	return d
}

func (d Data) Merge(other Data) Data {
	for key, value := range other {
		d[key] = value
	}
	return d
}
