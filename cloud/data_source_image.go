package cloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceImage() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceImageRead,

		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Optional: true,
				Description: "It's recommended not to set this field, and instead use either os/code combination " +
					"for public images or private_image_name for private images.",
			},
			"datacenter_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id field of datacenter data source",
			},
			"os": {
				Type:     schema.TypeString,
				Optional: true,
				Description: "Image OS, to see available OS, set the datacenter_id and run terraform plan, " +
					"it will show the list of available image OS/code combinations.",
			},
			"code": {
				Type:     schema.TypeString,
				Optional: true,
				Description: "Image code, to see available codes, set the datacenter_id and run terraform plan, " +
					"it will show the list of available image OS/code combinations.",
			},
			"private_image_name": {
				Type:     schema.TypeString,
				Optional: true,
				Description: "Private image name from Kamatare Console -> Hard Disk Library -> My Private Images. " +
					"Must not set os / code when specifing a private image. " +
					"Selected private image must be available in the given datacenter_id.",
			},
		},
	}
}

func getImageMatchesBy(images map[string]map[string]string, attr string, value string) (matchIDs []string) {
	if value == "" {
		return
	}

	for imageId, image := range images {
		if image[attr] == value {
			matchIDs = append(matchIDs, imageId)
		}
	}

	return
}

func getAvailableImages(images map[string]map[string]string) string {
	var availableImages []string
	availableImages = append(
		availableImages,
		fmt.Sprintf("%-10s %-30s %s", "os", "code", "name"),
	)
	for _, image := range images {
		availableImages = append(
			availableImages,
			fmt.Sprintf(
				"%-10s %-30s %s",
				"\""+image["os"]+"\"",
				"\""+image["code"]+"\"",
				"\""+image["name"]+"\"",
			),
		)
	}
	return strings.Join(availableImages, "\n")
}

func dataSourceImageRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	datacenterId := d.Get("datacenter_id").(string)
	os := d.Get("os").(string)
	code := d.Get("code").(string)
	privateImageName := d.Get("private_image_name").(string)
	if privateImageName == "" {
		provider := m.(*ProviderConfig)
		result, err := request(provider, "GET", fmt.Sprintf("service/server?images=1&datacenter=%s", datacenterId), nil)
		if err != nil {
			d.SetId("")
			return diag.FromErr(err)
		}
		images := map[string]map[string]string{}
		for _, image := range result.([]interface{}) {
			images[image.(map[string]interface{})["id"].(string)] = map[string]string{
				"os":   image.(map[string]interface{})["os"].(string),
				"code": image.(map[string]interface{})["code"].(string),
				"name": image.(map[string]interface{})["name"].(string),
			}
		}
		image, hasImage := images[id]
		osImageIds := getImageMatchesBy(images, "os", os)
		codeImageIds := getImageMatchesBy(images, "code", code)
		if hasImage &&
			(len(osImageIds) == 0 || (len(osImageIds) == 1 && osImageIds[0] == image["id"] && image["os"] == os)) &&
			(len(codeImageIds) == 0 || (len(codeImageIds) == 1 && codeImageIds[0] == image["id"] && image["code"] == code)) {
			d.SetId(image["id"])
			d.Set("code", image["code"])
			d.Set("os", image["os"])
			return nil
		} else if len(osImageIds) == 1 &&
			(!hasImage || image["os"] == os) &&
			(len(codeImageIds) == 0 || (len(codeImageIds) == 1 && codeImageIds[0] == osImageIds[0])) {
			d.SetId(osImageIds[0])
			d.Set("code", images[osImageIds[0]]["code"])
			d.Set("os", os)
			return nil
		} else if len(codeImageIds) == 1 &&
			(!hasImage || image["code"] == code) {
			d.SetId(codeImageIds[0])
			d.Set("code", code)
			d.Set("os", images[codeImageIds[0]]["os"])
			return nil
		} else {
			d.SetId("")
			d.Set("code", "")
			d.Set("os", "")
			return diag.Errorf("could not find matching image, available public images: \n"+
				"%s\n\n"+
				"Private images are not listed, see the following link for details: https://docs.cloud.ims-network.net", getAvailableImages(images))
		}
	} else {
		if code != "" || os != "" {
			return diag.Errorf("When specifying private_image_name, other attributes must not be set")
		} else {
			d.SetId(privateImageName)
			return nil
		}
	}
}
