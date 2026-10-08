package main
import "fmt"
type CountryCode string

const (
	Unknown CountryCode = ""
	India CountryCode = "in"
	USA CountryCode = "us"
)

func createCountryCode(c CountryCode){
	if c == Unknown{
		fmt.Println("No CC") 
	}else{
		fmt.Println("CC",c)
	}
}

// This non-generc approach
// Generic approach will have everything refer Enum below except Values()
type Role struct{
	role string
}

func (Role) Values()[]string{
	return []string{
		"Admin","Member",
	}
}

func (r *Role)Unmarshal(t []byte)error{
	for _,v :=range r.Values(){
		if v== string(t){
			r.role = v
			return nil
		}
	}
	return fmt.Errorf("no role defined")
}

func (r Role) String() string{
	return r.role
}
//

type RoleG struct{
	Enum[RoleType]
}

type RoleType struct{
}

func (RoleType) Values()[]string{
	return []string{
		"Admin","Member",
	}
}

// var (
// 	NoRole = Role {""}
// 	Admin  = Role {"Admin"}
// 	Member = Role {"Member"}
// )

// func FromString(s string)(Role,error){
// 	switch s{
// 	default:
// 		return Role{}, fmt.Errorf("No role")
// 	case Admin.role:
// 		return Admin, nil
// 	case Member.role:
// 		return Member, nil
// 	}
// }

func main(){
	createCountryCode(India)
	createCountryCode("abc")
	createCountryCode("")
	var s States
	_ = s.Unmarshal([]byte("MP"))
	fmt.Println(s)   // "MP"

	var se States
	err:= se.Unmarshal([]byte("XX"))
	if err!=nil{
		fmt.Println(err.Error()) 
	}
}

type Enumerable interface{
	Values() []string
}

type Enum[T Enumerable] struct{
	value string
}

func (e *Enum[T]) Unmarshal(text []byte)error{
	var enum T
	for _,v:=range enum.Values(){
		if v == string(text){
			e.value = v
			return nil
		}
	}
	return fmt.Errorf("invalid value %q, expected %v", text, enum.Values())
}
func (e Enum[T]) String() string {
    return e.value
}

type StatesType struct{}

func (StatesType)Values() []string{
	return []string{
		"MP",
		"UP",
		"AP",
	}
}

type States struct{
	Enum[StatesType]
}


// Make it more generic
type EnumerableG[V compareable] interface{
	Values() []V
}

type Enum[T EnumerableG[V], V compareable] struct{
	value V
}

func (e *Enum[T,V]) UnmarshalJSON(data []byte) error {
	var v V

	if err:=json.Unmarshal(data,&v);err!=nil{
		return err
	}

	var enum T
	for _,val:=range enum.Values(){
		if v == val{
			e.value = v
			return nil
		}
	}
	return fmt.Errorf("invalid value %q, expected %v", text, enum.Values())
}

func (e Enum[T,V]) String() string {
    return fmt.Sprinf(e.value)
}