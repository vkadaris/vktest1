package main

type SimpleClass struct {
  Name string
  Value int
}

func (s SimpleClass) GetName() string {
  return s.Name
}
