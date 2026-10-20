package util

// ClampProductQuantity brings a pack count into the range a product may hold:
// at least one pack and never more than MaxProductQuantity. A count of zero
// means "not given" on the paths where the field is optional, so it is stored
// as one pack.
func ClampProductQuantity(quantity int) int {
	if quantity < MinProductQuantity {
		return MinProductQuantity
	}
	if quantity > MaxProductQuantity {
		return MaxProductQuantity
	}
	return quantity
}
