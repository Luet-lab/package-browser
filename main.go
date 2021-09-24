package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/Masterminds/sprig"
	config "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/narqo/go-badge"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

var (
	CLIVersion = ""
)

var Repositories installer.Repositories

func refreshRepositories(repos installer.Repositories) (installer.Repositories, error) {
	syncedRepos := installer.Repositories{}
	for _, r := range repos {
		repo, err := r.Sync(false)
		if err != nil {
			return nil, errors.Wrap(err, "Failed syncing repository: "+r.GetName())
		}
		syncedRepos = append(syncedRepos, repo)
	}

	// compute what to install and from where
	sort.Sort(syncedRepos)

	return syncedRepos, nil
}

func GetRepo(name, url, t string) (*installer.LuetSystemRepository, error) {
	if t == "" {
		t = "http"
	}
	return installer.NewLuetSystemRepositoryFromYaml([]byte(`
name: "`+name+`"
type: "`+t+`"
urls:
- "`+url+`"`), pkg.NewInMemoryDatabase(false))
}

type Repository struct {
	Name, Url, Type, Github, Description string
}
type Meta struct {
	Repositories []Repository
}

func syncRepos(repos installer.Repositories) {

	dir, err := ioutil.TempDir(os.TempDir(), "example")
	if err != nil {
		fmt.Println("failed refreshing repository", err)
	}
	defer os.RemoveAll(dir)

	config.LuetCfg.System.TmpDirBase = dir
	config.LuetCfg.GetLogging().Color = false
	config.LuetCfg.GetGeneral().Debug = true
	InitAurora()
	repos, err = refreshRepositories(repos)
	if err != nil {
		fmt.Println("failed refreshing repository", err)
	}
	Repositories = repos
}

func render(tpl string, data interface{}) string {
	b := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("template").Funcs(sprig.FuncMap()).Parse(tpl))

	err := t.Execute(b, data)
	if err != nil {
		fmt.Printf("Error during template execution: %s", err)
		return b.String()
	}
	return b.String()
}
func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

func renderAll(configFile, outputDir, templatesDir string) {

	var metadata Meta

	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(fmt.Sprintf("yamlFile.Get err   #%v ", err))
	}
	err = yaml.Unmarshal(yamlFile, &metadata)
	if err != nil {
		panic(fmt.Sprintf("Unmarshal err   #%v ", err))
	}
	rawData := map[string]interface{}{}
	err = yaml.Unmarshal(yamlFile, &rawData)
	if err != nil {
		panic(fmt.Sprintf("Unmarshal err   #%v ", err))
	}
	additionalData := map[string]map[string]string{}

	repos := installer.Repositories{}
	for _, r := range metadata.Repositories {
		repo, err := GetRepo(r.Name, r.Url, r.Type)
		if err != nil {
			fmt.Println("Failed getting repo ", repo, err)
			continue
		}
		additionalData[r.Name] = make(map[string]string)
		additionalData[r.Name]["github"] = r.Github
		additionalData[r.Name]["description"] = r.Description
		additionalData[r.Name]["url"] = r.Url
		additionalData[r.Name]["type"] = r.Type
		repos = append(repos, repo)
	}

	syncRepos(repos)

	os.MkdirAll(outputDir, os.ModePerm)

	allCats := map[string]map[string]map[string][]pkg.Package{}
	allPacks := map[string][]pkg.Package{}

	for _, r := range Repositories {

		data := map[string]interface{}{}
		repoDir := filepath.Join(outputDir, r.Name)

		// Render packages in a repository
		packs := r.GetTree().GetDatabase().World()
		sort.SliceStable(packs, func(i, j int) bool {
			return packs[i].GetName() < packs[j].GetName()
		})
		data["Packages"] = packs
		data["AdditionalData"] = additionalData
		data["RepositoryName"] = r.Name
		data["Config"] = rawData
		dat, err := ioutil.ReadFile(filepath.Join(templatesDir, "repository.tmpl"))
		checkErr(err)

		str := render(string(dat), data)
		os.MkdirAll(repoDir, os.ModePerm)
		ioutil.WriteFile(filepath.Join(repoDir, "index.html"), []byte(str), os.ModePerm)

		// render badges for the repository
		os.MkdirAll(filepath.Join(outputDir, "badge"), os.ModePerm)
		badge, err := badge.RenderBytes(strconv.Itoa(len(r.GetIndex())), r.Name, "#3C1")
		checkErr(err)

		ioutil.WriteFile(filepath.Join(outputDir, "badge", r.Name), badge, os.ModePerm)

		// Print all individual categories for the repository
		// construct cats and allcats
		cats := map[string]map[string][]pkg.Package{}
		for _, p := range packs {
			if _, ok := cats[p.GetCategory()]; !ok {
				cats[p.GetCategory()] = make(map[string][]pkg.Package)
			}
			if _, ok := allCats[p.GetCategory()]; !ok {
				allCats[p.GetCategory()] = make(map[string]map[string][]pkg.Package)
			}
			if _, ok := allCats[p.GetCategory()][p.GetName()]; !ok {
				allCats[p.GetCategory()][p.GetName()] = make(map[string][]pkg.Package)
			}

			cats[p.GetCategory()][p.GetName()] = append(cats[p.GetCategory()][p.GetName()], p)
			// that will be used later to print all packages in a repository
			allCats[p.GetCategory()][p.GetName()][r.Name] = append(allCats[p.GetCategory()][p.GetName()][r.Name], p)
		}

		// will be used for the index
		allPacks[r.Name] = packs

		for c, pn := range cats {
			for ppn, p := range pn {

				pks := map[string][]pkg.Package{}
				pks[r.Name] = append(pks[r.Name], p...)

				data := map[string]interface{}{}

				data["PackageCategory"] = c
				data["PackageName"] = ppn
				data["Packages"] = pks
				data["Config"] = rawData
				dat, err := ioutil.ReadFile(filepath.Join(templatesDir, "packages.tmpl"))
				checkErr(err)
				str := render(string(dat), data)

				os.MkdirAll(filepath.Join(repoDir, c, ppn), os.ModePerm)
				ioutil.WriteFile(filepath.Join(repoDir, c, ppn, "index.html"), []byte(str), os.ModePerm)
			}
		}

		// All packages
		for _, p := range packs {
			data := map[string]interface{}{}
			data["Files"] = []string{}
			for _, a := range r.GetIndex() {
				if a.CompileSpec.GetPackage().GetFingerPrint() == p.GetFingerPrint() {
					data["Files"] = a.Files
				}
			}
			data["RepositoryName"] = r.Name
			data["Package"] = p
			data["Config"] = rawData
			dat, err := ioutil.ReadFile(filepath.Join(templatesDir, "package.tmpl"))
			checkErr(err)
			str := render(string(dat), data)

			os.MkdirAll(filepath.Join(repoDir, p.GetCategory(), p.GetName(), p.GetVersion()), os.ModePerm)
			ioutil.WriteFile(filepath.Join(repoDir, p.GetCategory(), p.GetName(), p.GetVersion(), "index.html"), []byte(str), os.ModePerm)
		}

	}

	// Generate all packages grouped by repositories
	for c, pn := range allCats {
		for ppn, p := range pn {
			pks := map[string][]pkg.Package{}

			for repo, packages := range p {
				pks[repo] = append(pks[repo], packages...)
			}

			data := map[string]interface{}{}

			data["PackageCategory"] = c
			data["PackageName"] = ppn
			data["Packages"] = pks
			data["Config"] = rawData
			dat, err := ioutil.ReadFile(filepath.Join(templatesDir, "packages.tmpl"))
			checkErr(err)
			str := render(string(dat), data)

			os.MkdirAll(filepath.Join(outputDir, "find", c, ppn), os.ModePerm)
			ioutil.WriteFile(filepath.Join(outputDir, "find", c, ppn, "index.html"), []byte(str), os.ModePerm)
		}
	}

	// Generate index
	data := map[string]interface{}{}
	data["Repositories"] = Repositories
	data["AdditionalData"] = additionalData
	data["Packages"] = allPacks
	data["Config"] = rawData
	
	dat, err := ioutil.ReadFile(filepath.Join(templatesDir, "index.tmpl"))
	checkErr(err)
	str := render(string(dat), data)
	ioutil.WriteFile(filepath.Join(outputDir, "index.html"), []byte(str), os.ModePerm)
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "config.yaml",
				Usage:   "config file",
				EnvVars: []string{"CONFIG"},
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "build",
				Usage:   "output directory",
				EnvVars: []string{"OUTPUT"},
			},
			&cli.StringFlag{
				Name:    "templates",
				Aliases: []string{"t"},
				Value:   "/usr/share/luet-package-browser",
				Usage:   "templates directory",
				EnvVars: []string{"TEMPLATES_DIR"},
			},
		},
		Name:        "Package browser",
		Usage:       "render packages websites",
		Description: "Generate static HTML for browsing packages in luet repositories",
		Action: func(c *cli.Context) error {
			renderAll(c.String("config"), c.String("output"), c.String("templates"))
			return nil
		},
		Version: CLIVersion,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
