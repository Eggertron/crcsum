import argparse
import json
import os
import zlib

__version__ = "1.0.0"

# https://stackoverflow.com/a/58141165
def crc32(fileName):
    with open(fileName, 'rb') as fh:
        hash = 0x0
        while True:
            s = fh.read(65536)
            if not s:
                break
            hash = zlib.crc32(s, hash)
        return hash

def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("path", help="path to crc; or json file to read")
    parser.add_argument("-o", "--output", default=None, help="save output json")
    parser.add_argument("-r", "--read", action="store_true", default=False, help="reads a crc json file")
    parser.add_argument("-R", "--recursive", action="store_true", default=False, help="recursivly scan folders")
    parser.add_argument("-p", "--pretty-output", action="store_true", default=False)
    return parser.parse_args()
    
def main():
    if os.name == 'nt':
        print("Windows GUI mode")
        from windows_gui import get_args_gooey
        args = get_args_gooey()
    else:
        args = get_args()
    if args.read:
        print(f"reading crc file {args.path}")
        with open(args.path, 'r') as fd:
            data = json.loads(fd.read())
        files = data.get("files")
        for file in files:
            c = file.get("crc")
            f = file.get("filename")
            crc = crc32(f)
            print(f"{'OK' if crc == c else 'XX'} : {c} == {crc} : {f}")
        return
    files = []
    data = {"files":files, "version":__version__, "zlib_version":zlib.__version__}
    if os.path.isfile(args.path):
        full_paths = [args.path]
    elif os.path.isdir(args.path):
        if args.recursive:
            full_paths = [os.path.join(dirpath,f) for (dirpath, dirnames, filenames) in os.walk(args.path) for f in filenames]
        else:
            full_paths = [os.path.join(args.path, p) for p in os.listdir(args.path)]
    else:
        print(f"unknown file type {args.path}")
        return
    for path in full_paths:
        if os.path.isfile(path):
            crc = crc32(path)
            files.append({
                "filename": path,
                "crc": crc
            })
        print(f"{crc} : {path}")
    if args.output:
        with open(args.output, 'w') as fd:
            if args.pretty_output:
                fd.write(json.dumps(data, indent=4))
            else:
                fd.write(json.dumps(data))
    #print(json.dumps(data, indent=4))

if __name__ == "__main__":
    main()