package view

const (
	DefaultPerPage = 10
	defaultWindow  = 5
)

type Page struct {
	Enabled bool
	Number  int
	PerPage int
	Total   int64
	Pages   int
	Offset  int
	Limit   int
	HasPrev bool
	HasNext bool
	Prev    int
	Next    int
	First   int
	Last    int
	Numbers []int
}

func (p Page) Empty() bool { return p.Total == 0 }

func (p Page) Single() bool { return p.Pages <= 1 }

type Paginator interface {
	Paginate(total int64, number, perPage int) Page
}

type Pages struct {
	Window int
}

func (d Pages) Paginate(total int64, number, perPage int) Page {
	if perPage < 1 {
		return Page{Number: 1, Total: total, Pages: 1, First: 1, Last: 1}
	}
	if number < 1 {
		number = 1
	}

	count := int(total / int64(perPage))
	if total%int64(perPage) != 0 {
		count++
	}
	if count < 1 {
		count = 1
	}
	if number > count {
		number = count
	}

	page := Page{
		Enabled: true,
		Number:  number,
		PerPage: perPage,
		Total:   total,
		Pages:   count,
		Offset:  (number - 1) * perPage,
		Limit:   perPage,
		HasPrev: number > 1,
		HasNext: number < count,
		First:   1,
		Last:    count,
	}
	page.Prev, page.Next = number-1, number+1
	if !page.HasPrev {
		page.Prev = 1
	}
	if !page.HasNext {
		page.Next = count
	}
	page.Numbers = d.window(number, count)
	return page
}

func (d Pages) window(number, count int) []int {
	size := d.Window
	if size < 1 {
		size = defaultWindow
	}
	if size > count {
		size = count
	}

	start := number - size/2
	if start < 1 {
		start = 1
	}
	if start+size-1 > count {
		start = count - size + 1
	}

	out := make([]int, 0, size)
	for i := start; i < start+size; i++ {
		out = append(out, i)
	}
	return out
}

func Paginate(paginator Paginator, total int64, number, perPage int) Page {
	if perPage < 1 {
		return Page{Number: 1, Total: total, Pages: 1, First: 1, Last: 1}
	}
	if paginator == nil {
		paginator = Pages{}
	}
	return paginator.Paginate(total, number, perPage)
}
