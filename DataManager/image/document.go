package image

//func mupdf(file io.Reader, mime, format string, size, quality int) (*bytes.Buffer, error) {
//	tmpdir, err := ioutil.TempDir("", "pbooru-tmp")
//	if err != nil {
//		log.Println(err)
//		return nil, err
//
//	}
//	defer os.RemoveAll(tmpdir)
//
//	var tmpbuf bytes.Buffer
//	tmpbuf.ReadFrom(file)
//
//	err = ioutil.WriteFile(fmt.Sprintf("%s/file.%s", tmpdir, mime), tmpbuf.Bytes(), 0660)
//
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//
//	args := []string{
//		"draw",
//		"-o",
//		"",
//		"-F",
//		"png",
//		fmt.Sprintf("%s/file.%s", tmpdir, mime),
//		"1",
//	}
//
//	cmd := exec.Command("mutool", args...)
//
//	var b, er bytes.Buffer
//	cmd.Stdout = &b
//	cmd.Stderr = &er
//
//	err = cmd.Run()
//	if err != nil {
//		log.Println(b.String(), er.String(), err)
//		return nil, err
//	}
//
//	f := bytes.NewReader(b.Bytes())
//	return magickResize(f, format, size, quality)
//}
//
//func gnomeMobi(file io.Reader, format string, size, quality int) (*bytes.Buffer, error) {
//	tmpdir, err := ioutil.TempDir("", "pbooru-tmp")
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//	defer os.RemoveAll(tmpdir)
//
//	var tmpbuf bytes.Buffer
//	tmpbuf.ReadFrom(file)
//
//	err = ioutil.WriteFile(fmt.Sprintf("%s/file.%s", tmpdir, "mobi"), tmpbuf.Bytes(), 0660)
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//
//	args := []string{
//		"-s",
//		strconv.Itoa(2048),
//		fmt.Sprintf("%s/file.mobi", tmpdir),
//		fmt.Sprintf("%s/out.png", tmpdir),
//	}
//
//	cmd := exec.Command("gnome-mobi-thumbnailer", args...)
//
//	var b, er bytes.Buffer
//	cmd.Stdout = &b
//	cmd.Stderr = &er
//
//	err = cmd.Run()
//	if err != nil {
//		log.Println(b.String(), er.String(), err)
//		return nil, err
//	}
//
//	f, err := os.Open(filepath.Join(tmpdir, "out.png"))
//
//	if err != nil {
//		log.Println(err)
//		return nil, err
//	}
//
//	defer f.Close()
//
//	return magickResize(f, format, size, quality)
//}
