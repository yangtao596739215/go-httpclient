package utils

import "bytes"

func setSubtract(setA, setB []string) []string {
	result := []string{}

	for _, a := range setA {
		found := false
		for _, b := range setB {
			if bytes.Equal([]byte(a), []byte(b)) {
				found = true
				break
			}
		}

		if !found {
			result = append(result, a)
		}
	}

	return result
}

//返回新列表相比于当前列表，需要新增或减少的东西
func AddrListDiff(curList, newList []string) (addList, delList []string) {
	addList = setSubtract(newList, curList)

	delList = setSubtract(curList, newList)

	return addList, delList
}

func MakeUrl(addr, path string) string {
	url := "http://" + addr + "/" + path
	return url
}
