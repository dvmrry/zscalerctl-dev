def unexpected_member_keys:
  [
    .[]
    | (.locations // [])[]
    | (keys_unsorted - ["id", "name"])[]
  ]
  | unique;

unexpected_member_keys as $unexpected
| if any(.[]; has("lastModUser")) then
    error("forbidden lastModUser field present")
  elif ($unexpected | length) != 0 then
    error("unexpected location member field(s): \($unexpected | join(", "))")
  else
    [
      .[]
      | {
          id,
          name,
          groupType,
          member_count: ((.locations // []) | length),
          members: [(.locations // [])[] | {id, name}]
        }
    ]
  end
