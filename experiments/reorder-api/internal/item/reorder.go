package item

func reorder(current []Item, id string, previousID *string) ([]Item, error) {
	items := append([]Item(nil), current...)
	movingIndex := indexByID(items, id)
	if movingIndex == -1 {
		return nil, ErrItemNotFound
	}
	if previousID != nil {
		if id == *previousID {
			return nil, ErrPreviousSame
		}
		if indexByID(items, *previousID) == -1 {
			return nil, ErrItemNotFound
		}
	}

	movingItem := items[movingIndex]
	items = append(items[:movingIndex], items[movingIndex+1:]...)

	insertIndex := 0
	if previousID != nil {
		insertIndex = indexByID(items, *previousID) + 1
	}

	items = append(items, Item{})
	copy(items[insertIndex+1:], items[insertIndex:])
	items[insertIndex] = movingItem

	for i := range items {
		items[i].Position = i + 1
	}
	return items, nil
}

func indexByID(items []Item, id string) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}
