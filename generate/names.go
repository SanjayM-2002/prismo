package generate

import (
	"fmt"

	"github.com/SanjayM-2002/prismo/schema"
)

// reservedNames are top-level names the generated client always declares.
var reservedNames = []string{"Client", "Open", "NewClient", "ErrNotFound", "Ptr"}

// modelSuffixes are added to a model's Go name to make the other top-level
// names generated for it: User -> User, UserClient, UserCreate, UserWhere, UserUpdate.
var modelSuffixes = []string{"", "Client", "Create", "Where", "Update"}

// checkNames returns an error if two top-level names in the generated
// package would clash.
func checkNames(s *schema.Schema) error {
	owners := map[string]string{}
	for _, name := range reservedNames {
		owners[name] = ""
	}

	for _, m := range s.Models {
		base := goName(m.Name)
		for _, suffix := range modelSuffixes {
			ident := base + suffix
			if owner, ok := owners[ident]; ok {
				if owner == "" {
					return fmt.Errorf("model %s: generated name %s is reserved by the client", m.Name, ident)
				}
				return fmt.Errorf("models %s and %s both generate the name %s", owner, m.Name, ident)
			}
			owners[ident] = m.Name
		}
	}
	return nil
}
