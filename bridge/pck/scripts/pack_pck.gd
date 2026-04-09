extends SceneTree

func _init() -> void:
    var args := OS.get_cmdline_user_args()
    if args.size() < 2:
        printerr("Usage: godot_console --headless --path <project> --script res://scripts/pack_pck.gd -- <src_dir> <output_pck>")
        quit(1)
        return

    var source_dir := args[0]
    var output_pck := args[1]
    var result := _pack(source_dir, output_pck)
    quit(result)


func _pack(source_dir: String, output_pck: String) -> int:
    var packer := PCKPacker.new()
    var err := packer.pck_start(output_pck)
    if err != OK:
        printerr("Failed to start PCK: ", err)
        return err

    err = _add_directory(packer, source_dir, "")
    if err != OK:
        return err

    err = packer.flush()
    if err != OK:
        printerr("Failed to flush PCK: ", err)
    return err


func _add_directory(packer: PCKPacker, source_dir: String, relative_path: String) -> int:
    var dir := DirAccess.open(source_dir)
    if dir == null:
        printerr("Could not open source dir: ", source_dir)
        return ERR_CANT_OPEN

    dir.list_dir_begin()
    while true:
        var name := dir.get_next()
        if name == "":
            break
        if name == "." or name == "..":
            continue

        var absolute_path := source_dir.path_join(name)
        var child_relative := name if relative_path == "" else relative_path.path_join(name)
        if dir.current_is_dir():
            var nested_err := _add_directory(packer, absolute_path, child_relative)
            if nested_err != OK:
                return nested_err
            continue

        var resource_path := "res://%s" % child_relative.replace("\\", "/")
        var add_err := packer.add_file(resource_path, absolute_path)
        if add_err != OK:
            printerr("Failed to add file to PCK: ", absolute_path, " -> ", resource_path, " (", add_err, ")")
            return add_err

    return OK
