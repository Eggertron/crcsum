import argparse
import json
import os
import zlib

__version__ = "1.0.0"

# https://stackoverflow.com/a/58141165
def crc32(fileName):
    if not os.path.exists(fileName):
        print(f"ERROR: {fileName} not found.")
        return 0
    if not os.path.isfile(fileName):
        print(f"ERROR: {fileName} is not a valid file.")
        return 0
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
    parser.add_argument("path", help="path to crc; or json file to read; or file/folder to checksum.")
    parser.add_argument("-o", "--output", default=None, help="save output json")
    parser.add_argument("-r", "--read", action="store_true", default=False, help="reads a crc json file")
    parser.add_argument("-R", "--recursive", action="store_true", default=False, help="recursivly scan folders")
    parser.add_argument("-p", "--pretty-output", action="store_true", default=False)
    return parser.parse_args()
    
def main():
    if os.name == 'nt':
        from windows_gui import get_args_gooey
        args = get_args_gooey()
    else:
        args = get_args()
    real_dirname = os.path.realpath(args.path)
    if not os.path.isdir(real_dirname):
        real_dirname = os.path.dirname(real_dirname)
    # fix windows path
    #if os.name == 'nt':
    #    real_dirname = real_dirname.encode('unicode_escape')
    os.chdir(real_dirname)  # move to the folder where the work will be done.
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
    if args.recursive:
        full_paths = [os.path.join(dirpath,f) for (dirpath, dirnames, filenames) in os.walk('.') for f in filenames]
    elif os.path.isfile(args.path):
        full_paths = [args.path]
    elif os.path.isdir(args.path):
        full_paths = [p for p in os.listdir() if not os.path.isdir(os.path.join(args.path, p))]
    else:
        print(f"unknown file type {args.path}")
        return
    files_total = len(full_paths)
    files_count = 1
    for path in full_paths:
        if os.path.isfile(path):
            crc = crc32(path)
            files.append({
                "filename": path,
                "crc": crc
            })
        print(f"Processed [{str(files_count)} / {str(files_total)}] ({str(files_count/files_total*100)}%)")
        files_count += 1
    print(json.dumps(files, indent=4))
    if args.output:
        out_filepath = os.path.join(real_dirname, args.output)
        if os.path.exists(out_filepath):
            if input(f"The file path {out_filepath} already exists. Do you want to overwrite? (y/N) : ").lower() != 'y':
                print("Cancelled.")
                return
        with open(out_filepath, 'w') as fd:
            if args.pretty_output:
                fd.write(json.dumps(data, indent=4))
            else:
                fd.write(json.dumps(data))
    #print(json.dumps(data, indent=4))

if __name__ == "__main__":
    main()
