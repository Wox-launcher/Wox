"""Exercise the production material shader without opening a desktop window.

Run with python3; requires cc, EGL/OpenGL development files, and surfaceless EGL.
"""
from pathlib import Path
import subprocess
import tempfile

source = (Path(__file__).resolve().parents[1] / "native_linux.c").read_text()
shaders = []
for name in ("rect_vertex_source", "compose_fragment_source"):
    start = source.index("static const char *const " + name + " =")
    end = source.index('\n\n', start)
    shaders.append(source[start:end])

harness = r"""
#define GL_GLEXT_PROTOTYPES
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GL/glcorearb.h>
#include <assert.h>
#include <math.h>
#include <stdio.h>
""" + "\n".join(shaders) + r"""
static GLuint compile(GLenum type, const char *source) {
  GLuint shader = glCreateShader(type);
  glShaderSource(shader, 1, &source, NULL);
  glCompileShader(shader);
  GLint ok; glGetShaderiv(shader, GL_COMPILE_STATUS, &ok);
  if (!ok) { char log[4096]; glGetShaderInfoLog(shader, sizeof(log), NULL, log); puts(log); }
  assert(ok);
  return shader;
}
int main(void) {
  EGLDisplay display = eglGetPlatformDisplay(EGL_PLATFORM_SURFACELESS_MESA, EGL_DEFAULT_DISPLAY, NULL);
  assert(eglInitialize(display, NULL, NULL));
  assert(eglBindAPI(EGL_OPENGL_API));
  EGLint attrs[] = {EGL_SURFACE_TYPE, EGL_PBUFFER_BIT, EGL_RENDERABLE_TYPE, EGL_OPENGL_BIT, EGL_NONE};
  EGLConfig config; EGLint count;
  assert(eglChooseConfig(display, attrs, &config, 1, &count) && count);
  EGLint context_attrs[] = {EGL_CONTEXT_MAJOR_VERSION, 3, EGL_CONTEXT_MINOR_VERSION, 3, EGL_NONE};
  EGLContext context = eglCreateContext(display, config, EGL_NO_CONTEXT, context_attrs);
  assert(context != EGL_NO_CONTEXT && eglMakeCurrent(display, EGL_NO_SURFACE, EGL_NO_SURFACE, context));
  GLuint program = glCreateProgram();
  glAttachShader(program, compile(GL_VERTEX_SHADER, rect_vertex_source));
  glAttachShader(program, compile(GL_FRAGMENT_SHADER, compose_fragment_source));
  glLinkProgram(program); GLint ok; glGetProgramiv(program, GL_LINK_STATUS, &ok); assert(ok);
  glUseProgram(program);
  GLuint vao; glGenVertexArrays(1, &vao); glBindVertexArray(vao);
  GLuint textures[3]; glGenTextures(3, textures);
  float color[] = {0.4f, 0.1f, 0.0f, 0.5f};
  for (int i = 0; i < 2; i++) {
    glActiveTexture(GL_TEXTURE0+i); glBindTexture(GL_TEXTURE_2D, textures[i]);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA32F, 1, 1, 0, GL_RGBA, GL_FLOAT, color);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST);
  }
  glActiveTexture(GL_TEXTURE2); glBindTexture(GL_TEXTURE_2D, textures[2]);
  glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, 8, 8, 0, GL_RGBA, GL_UNSIGNED_BYTE, NULL);
  GLuint fbo; glGenFramebuffers(1, &fbo); glBindFramebuffer(GL_FRAMEBUFFER, fbo);
  glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, textures[2], 0);
  assert(glCheckFramebufferStatus(GL_FRAMEBUFFER) == GL_FRAMEBUFFER_COMPLETE);
  glViewport(0, 0, 8, 8);
  glUniform1i(glGetUniformLocation(program,"u_original"),0);
  glUniform1i(glGetUniformLocation(program,"u_blurred"),1);
  glUniform2f(glGetUniformLocation(program,"u_viewport"),8,8);
  glUniform4f(glGetUniformLocation(program,"u_rect"),0,0,8,8);
  glUniform4f(glGetUniformLocation(program,"u_source"),0,0,8,8);
  glUniform1f(glGetUniformLocation(program,"u_scale"),1);
  glUniform1f(glGetUniformLocation(program,"u_radius"),3);
  float settings[][2] = {{1,1},{0.5f,0},{0,1},{2,2}};
  for (int i=0; i<4; i++) {
    glUniform1f(glGetUniformLocation(program,"u_brightness"),settings[i][0]);
    glUniform1f(glGetUniformLocation(program,"u_saturation"),settings[i][1]);
    glDrawArrays(GL_TRIANGLE_STRIP,0,4);
    unsigned char pixel[4], corner[4];
    glReadPixels(4,4,1,1,GL_RGBA,GL_UNSIGNED_BYTE,pixel);
    glReadPixels(0,0,1,1,GL_RGBA,GL_UNSIGNED_BYTE,corner);
    assert(abs(pixel[3]-128)<=1 && abs(corner[0]-102)<=1);
    for(int c=0;c<3;c++) assert(pixel[c]<=pixel[3]);
    if(i==0) assert(abs(pixel[0]-102)<=1 && abs(pixel[1]-26)<=1 && pixel[2]==0);
    if(i==1) assert(abs(pixel[0]-20)<=1 && pixel[0]==pixel[1] && pixel[1]==pixel[2]);
    if(i==2) assert(pixel[0]==0 && pixel[1]==0 && pixel[2]==0);
  }
  assert(glGetError()==GL_NO_ERROR);
  eglMakeCurrent(display,EGL_NO_SURFACE,EGL_NO_SURFACE,EGL_NO_CONTEXT);
  eglDestroyContext(display,context); eglTerminate(display);
  puts("material shader: brightness, saturation, alpha, and rounded clipping passed");
  return 0;
}
"""
with tempfile.TemporaryDirectory(prefix="wox-material-") as directory:
    path = Path(directory)
    (path / "test.c").write_text(harness.replace("#include <stdio.h>", "#include <stdio.h>\n#include <stdlib.h>"))
    subprocess.run(["cc", str(path / "test.c"), "-lEGL", "-lGL", "-lm", "-o", str(path / "test")], check=True)
    subprocess.run([str(path / "test")], check=True)
