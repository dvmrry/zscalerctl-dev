import {WireNumber, type WireValue} from "../../../clients/typescript/src/index.ts";

export const DEMO_DATA: WireValue = {
  product: "zia",
  resource: "locations",
  collected_at: "2026-07-14T22:30:00Z",
  records: [
    {
      id: new WireNumber("9007199254740993"),
      name: "Raleigh Headquarters",
      active: true,
      address: {
        city: "Raleigh",
        state: "NC",
        country: "US",
        coordinates: {
          latitude: new WireNumber("35.7796"),
          longitude: new WireNumber("-78.6382")
        }
      },
      network: {
        ip_addresses: ["198.51.100.18", "2001:db8:4::18"],
        bandwidth: {download_mbps: new WireNumber("1000"), upload_mbps: new WireNumber("500")},
        sub_locations: [
          {name: "Engineering", enabled: true, vlan: new WireNumber("120")},
          {name: "Guest", enabled: true, vlan: new WireNumber("220")}
        ]
      },
      groups: [
        {id: new WireNumber("31001"), name: "US Offices"},
        {id: new WireNumber("31008"), name: "Tier 1 Sites"}
      ],
      tags: ["headquarters", "east", "production"]
    },
    {
      id: new WireNumber("9007199254740995"),
      name: "Seoul Branch",
      active: true,
      address: {city: "서울", country: "KR"},
      network: {
        ip_addresses: ["203.0.113.42"],
        bandwidth: {download_mbps: new WireNumber("500"), upload_mbps: new WireNumber("500")},
        sub_locations: []
      },
      groups: [{id: new WireNumber("31021"), name: "APAC Offices"}],
      tags: ["branch", "apac"]
    }
  ],
  collection: {
    selected: true,
    status: "success",
    records: new WireNumber("2"),
    redaction: "standard"
  }
};
