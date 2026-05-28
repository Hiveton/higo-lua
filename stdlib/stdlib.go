package stdlib

type Profile struct {
	Base      bool
	Package   bool
	Coroutine bool
	Table     bool
	String    bool
	Math      bool
	IO        bool
	OS        bool
	Debug     bool
	FileLoad  bool
}

func Full() Profile {
	return Profile{Base: true, Package: true, Coroutine: true, Table: true, String: true, Math: true, IO: true, OS: true, Debug: true, FileLoad: true}
}

func Safe() Profile {
	return Profile{Base: true, Package: true, Coroutine: true, Table: true, String: true, Math: true}
}
