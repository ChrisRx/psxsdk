# Writes a TMD of a cube: eight vertices, six normals, six flat textured
# quads lit by the library, every face carrying the same 64x64 texture.
import struct, sys
h = 64
V = [(-h,-h,-h),(h,-h,-h),(h,h,-h),(-h,h,-h),(-h,-h,h),(h,-h,h),(h,h,h),(-h,h,h)]
# Each face: its outward normal and four corner indices in strip order
# (top-left, top-right, bottom-left, bottom-right seen from outside).
# The console's axes are X right, Y down and Z away, a left-handed set, so
# a face's corners read clockwise from outside when listed top-left,
# top-right, bottom-left, bottom-right with "top" towards -Y for the sides
# and towards +Z for the top and bottom.
faces = [
    ((0,0,-1), (0,1,3,2)),   # front, toward -Z
    ((0,0, 1), (5,4,6,7)),   # back
    ((-1,0,0), (4,0,7,3)),   # left
    (( 1,0,0), (1,5,2,6)),   # right
    ((0,-1,0), (4,5,0,1)),   # top, toward -Y, which is up on the screen
    ((0, 1,0), (3,2,7,6)),   # bottom
]
cba, tsb = 0x7800, 0x000a
uv = [(0,0),(63,0),(0,63),(63,63)]
prims = b''
for i, (n, c) in enumerate(faces):
    pkt = struct.pack('<BBH', uv[0][0], uv[0][1], cba) + struct.pack('<BBH', uv[1][0], uv[1][1], tsb)
    pkt += struct.pack('<BBH', uv[2][0], uv[2][1], 0) + struct.pack('<BBH', uv[3][0], uv[3][1], 0)
    pkt += struct.pack('<HHHHHH', i, c[0], c[1], c[2], c[3], 0)
    prims += bytes([9, 7, 0, 0x2c]) + pkt
verts = b''.join(struct.pack('<hhhh', *v, 0) for v in V)
norms = b''.join(struct.pack('<hhhh', n[0]*4096, n[1]*4096, n[2]*4096, 0) for n, _ in faces)
otab_len = 28
prim_off = otab_len; vert_off = prim_off + len(prims); norm_off = vert_off + len(verts)
otab = struct.pack('<7I', vert_off, len(V), norm_off, len(faces), prim_off, len(faces), 0)
out = struct.pack('<III', 0x41, 0, 1) + otab + prims + verts + norms
open(sys.argv[1], 'wb').write(out)
print(len(out), 'bytes')
