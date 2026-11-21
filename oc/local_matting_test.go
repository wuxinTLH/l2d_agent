package oc

import("os";"path/filepath";"testing")

func TestProvisionedLocalMatting(t *testing.T){
 if os.Getenv("LOCAL_MATTING_TEST")!="1"{t.Skip("requires provisioned local CPU model")}
 t.Chdir("..")
 input:=filepath.Join(".local","qa","matting-synthetic.png")
 out,err:=localMatte(input);if err!=nil{t.Fatal(err)}
 if out.Bounds().Dx()!=384||out.Bounds().Dy()!=320{t.Fatalf("unexpected canvas %v",out.Bounds())}
 if err:=autoSplitContent(out);err!=nil{t.Fatalf("local mask rejected: %v",err)}
}
