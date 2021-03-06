package main

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/urfave/cli"
)

type jsonContainer struct {
	ID            string `json:"id"`
	Builder       bool   `json:"builder"`
	ImageID       string `json:"imageid"`
	ImageName     string `json:"imagename"`
	ContainerName string `json:"containername"`
}

var (
	containersFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "also list non-buildah containers",
		},
		cli.BoolFlag{
			Name:  "json",
			Usage: "output in JSON format",
		},
		cli.BoolFlag{
			Name:  "noheading, n",
			Usage: "do not print column headings",
		},
		cli.BoolFlag{
			Name:  "notruncate",
			Usage: "do not truncate output",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "display only container IDs",
		},
	}
	containersDescription = "Lists containers which appear to be " + buildah.Package + " working containers, their\n   names and IDs, and the names and IDs of the images from which they were\n   initialized"
	containersCommand     = cli.Command{
		Name:        "containers",
		Usage:       "List working containers and their base images",
		Description: containersDescription,
		Flags:       containersFlags,
		Action:      containersCmd,
		ArgsUsage:   " ",
	}
)

func containersCmd(c *cli.Context) error {
	if err := validateFlags(c, containersFlags); err != nil {
		return err
	}
	store, err := getStore(c)
	if err != nil {
		return err
	}

	quiet := c.Bool("quiet")
	truncate := !c.Bool("notruncate")
	JSONContainers := []jsonContainer{}
	jsonOut := c.Bool("json")

	list := func(n int, containerID, imageID, image, container string, isBuilder bool) {
		if jsonOut {
			JSONContainers = append(JSONContainers, jsonContainer{ID: containerID, Builder: isBuilder, ImageID: imageID, ImageName: image, ContainerName: container})
			return
		}

		if n == 0 && !c.Bool("noheading") && !quiet {
			if truncate {
				fmt.Printf("%-12s  %-8s %-12s %-32s %s\n", "CONTAINER ID", "BUILDER", "IMAGE ID", "IMAGE NAME", "CONTAINER NAME")
			} else {
				fmt.Printf("%-64s %-8s %-64s %-32s %s\n", "CONTAINER ID", "BUILDER", "IMAGE ID", "IMAGE NAME", "CONTAINER NAME")
			}
		}
		if quiet {
			fmt.Printf("%s\n", containerID)
		} else {
			isBuilderValue := ""
			if isBuilder {
				isBuilderValue = "   *"
			}
			if truncate {
				fmt.Printf("%-12.12s  %-8s %-12.12s %-32s %s\n", containerID, isBuilderValue, imageID, image, container)
			} else {
				fmt.Printf("%-64s %-8s %-64s %-32s %s\n", containerID, isBuilderValue, imageID, image, container)
			}
		}
	}
	seenImages := make(map[string]string)
	imageNameForID := func(id string) string {
		if id == "" {
			return buildah.BaseImageFakeName
		}
		imageName, ok := seenImages[id]
		if ok {
			return imageName
		}
		img, err2 := store.Image(id)
		if err2 == nil && len(img.Names) > 0 {
			seenImages[id] = img.Names[0]
		}
		return seenImages[id]
	}

	builders, err := openBuilders(store)
	if err != nil {
		return errors.Wrapf(err, "error reading build containers")
	}
	if !c.Bool("all") {
		for i, builder := range builders {
			image := imageNameForID(builder.FromImageID)
			list(i, builder.ContainerID, builder.FromImageID, image, builder.Container, true)
		}
	} else {
		builderMap := make(map[string]struct{})
		for _, builder := range builders {
			builderMap[builder.ContainerID] = struct{}{}
		}
		containers, err2 := store.Containers()
		if err2 != nil {
			return errors.Wrapf(err2, "error reading list of all containers")
		}
		for i, container := range containers {
			name := ""
			if len(container.Names) > 0 {
				name = container.Names[0]
			}
			_, ours := builderMap[container.ID]
			list(i, container.ID, container.ImageID, imageNameForID(container.ImageID), name, ours)
		}
	}
	if jsonOut {
		data, err := json.MarshalIndent(JSONContainers, "", "    ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", data)
	}

	return nil
}
