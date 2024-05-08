package mapreduce

import "os"

func FileReader(name string, emit func(data any)) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	emit(string(data))
	return nil
}
